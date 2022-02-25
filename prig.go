package main

/*
Based on a similar idea for Nim:
https://github.com/c-blake/cligen/blob/master/examples/rp.nim

TODO:
- Consider making this a Go 1.18 showcase with Sort / SortMap
  + make it work with 1.17 but add optional Sort[T] and SortMap[T] with Go 1.18
  + or maybe make it use interface{} in Go 1.17 but type safe in Go 1.18
    - Sort(s interface{}) []interface{}
    - SortMap(m interface{}) []KV
- Add note about which packages are auto-imported? import math, strings, etc
  + or consider using goimports to do this automatically? test performance hit

*/

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

const version = "v0.1.0"

func main() {
	// Parse command line arguments
	if len(os.Args) <= 1 {
		errorf("%s", usage)
	}

	var begin []string
	var end []string
	var perRecord []string
	fieldSep := " "
	printSource := false

	for i := 1; i < len(os.Args); {
		arg := os.Args[i]
		i++

		switch arg {
		case "-b":
			if i >= len(os.Args) {
				errorf("-b requires an argument")
			}
			begin = append(begin, os.Args[i])
			i++
		case "-e":
			if i >= len(os.Args) {
				errorf("-e requires an argument")
			}
			end = append(end, os.Args[i])
			i++
		case "-F":
			if i >= len(os.Args) {
				errorf("-F requires an argument")
			}
			fieldSep = os.Args[i]
			i++
		case "-i":
			imports[os.Args[i]] = struct{}{}
			if i >= len(os.Args) {
				errorf("-e requires an argument")
			}
			i++
		case "-h", "--help":
			fmt.Println(usage)
			return
		case "-s":
			printSource = true
		case "-V", "--version":
			fmt.Println(version)
			return
		default:
			switch {
			case strings.HasPrefix(arg, "-F"):
				fieldSep = arg[2:]
			default:
				perRecord = append(perRecord, arg)
			}
		}
	}

	// Write source code to buffer
	var buffer bytes.Buffer
	params := &templateParams{
		FieldSep:    fieldSep,
		Imports:     imports,
		BeginID:     randomID(),
		Begin:       begin,
		PerRecordID: randomID(),
		PerRecord:   perRecord,
		EndID:       randomID(),
		End:         end,
	}
	err := sourceTemplate.Execute(&buffer, params)
	if err != nil {
		errorf("error executing template: %v", err)
	}
	sourceBytes := buffer.Bytes()

	if printSource {
		// TODO: go fmt it (or just let tools/imports do that?)
		fmt.Print(string(sourceBytes))
		return
	}

	// Create a temporary work directory and .go file
	tempDir, err := os.MkdirTemp("", "prig_")
	if err != nil {
		errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	goFilename := filepath.Join(tempDir, "main.go")
	err = os.WriteFile(goFilename, sourceBytes, 0666)
	if err != nil {
		errorf("error writing temp file: %v", err)
	}

	// Ensure that Go is installed
	_, err = exec.LookPath("go")
	if err != nil {
		errorf("You must install Go to use 'prig', see https://go.dev/doc/install")
	}

	// Build the program with "go build"
	exeFilename := filepath.Join(tempDir, "main")
	cmd := exec.Command("go", "build", "-o", exeFilename, goFilename)
	output, err := cmd.CombinedOutput()
	switch err.(type) {
	case nil:
	case *exec.ExitError:
		source, err := os.ReadFile(goFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading source file: %v", err)
		}
		parsed := parseErrors(string(output), string(source), params)
		fmt.Fprint(os.Stderr, parsed)
		os.Exit(1)
	default:
		errorf("error building program: %v", err)
	}

	// Then run the executable we just built
	cmd = exec.Command(exeFilename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		exitCode := cmd.ProcessState.ExitCode()
		if exitCode == -1 {
			errorf("error running program: %v", err)
		}
		os.Exit(exitCode)
	}
}

func randomID() string {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		errorf("error generating random bytes: %v", err)
	}
	h := sha1.Sum(b)
	return fmt.Sprintf("%x", h)
}

var compileErrorRe = regexp.MustCompile(`.*\.go:(\d+):(\d+): (.*)`)

func parseErrors(buildOutput string, source string, params *templateParams) string {
	var builder strings.Builder
	lines := strings.Split(buildOutput, "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		matches := compileErrorRe.FindStringSubmatch(line)
		if matches == nil {
			fmt.Fprintf(&builder, "%s\n", line)
			continue
		}
		lineNum, _ := strconv.Atoi(matches[1])
		colNum, _ := strconv.Atoi(matches[2])
		message := matches[3]
		builder.WriteString(formatError(source, params, lineNum, colNum, message))
		builder.WriteString("\n")
	}
	return builder.String()
}

func formatError(source string, params *templateParams, line, col int, message string) string {
	formatted := formatErrorChunk(source, params.BeginID, "begin", params.Begin, line, col, message)
	if formatted != "" {
		return formatted
	}
	formatted = formatErrorChunk(source, params.PerRecordID, "perrecord", params.PerRecord, line, col, message)
	if formatted != "" {
		return formatted
	}
	formatted = formatErrorChunk(source, params.EndID, "end", params.End, line, col, message)
	if formatted != "" {
		return formatted
	}
	return fmt.Sprintf("main.go:%d:%d: %s", line, col, message)
}

func formatErrorChunk(source, id, prefix string, chunk []string, line, col int, message string) string {
	pos := strings.Index(source, id)
	if pos < 0 {
		return fmt.Sprintf("main.go:%d:%d: %s", line, col, message)
	}
	curLine := strings.Count(source[:pos], "\n") + 2
	for i, block := range chunk {
		numLines := strings.Count(block, "\n") + 1
		if curLine+numLines > line {
			relLine := line - curLine + 1
			sourceLine := getSourceLine(block, relLine)
			caretLine := strings.Repeat(" ", col-1) + "^"
			return fmt.Sprintf("%s%d:%d:%d: %s\n%s\n%s", prefix, i+1, relLine, col, message, sourceLine, caretLine)
		}
		curLine += numLines
	}
	return ""
}

func getSourceLine(block string, line int) string {
	lines := strings.Split(block, "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

const usage = `Prig ` + version + ` - Copyright (c) 2022 Ben Hoyt

Prig is for Processing Records In Go. It's like AWK, but snobbish (Go! static
typing!). It runs 'begin code' first, then runs 'per-record code' for every
record (line) in the input, then runs 'end code'. Prig uses "go build", so it
requires the Go compiler: https://go.dev/doc/install

Usage: prig [options] [-b 'begin code'] 'per-record code' [-e 'end code']

Options:
  -b 'begin code'    Go code to run before processing input (multiple allowed)
  'per-record code'  Go code to run for every record of input; F(0) is record
  -e 'end code'      Go code to run after processing input (multiple allowed)

  -F char | re       field separator (single character or multi-char regex)
  -h, --help         print help message and exit
  -i import          add Go import
  -s                 print formatted Go source instead of running
  -V, --version      print version number and exit

Built-in functions:
  F(i int) string         // return field i (starts at 1; 0 is current record)
  Float(s string) float64 // convert string to float64 (or return 0.0)
  Int(s string) int       // convert string to int (or return 0)
  NF() int                // return number of fields in current record
  NR() int                // return number of current record

  Print(args ...interface{})                 // fmt.Print, but buffered
  Printf(format string, args ...interface{}) // fmt.Printf, but buffered
  Println(args ...interface{})               // fmt.Println, but buffered

  Match(re, s string) bool            // report whether s contains match of re
  Replace(re, s, repl string) string  // replace all re matches in s with repl
  Submatches(re, s string) []string   // return slice of submatches of re in s
  Substr(s string, n[, m] int) string // s[n:m] but safe and allow negative n/m

  SortInts(s []int[, options]) []int               // return new sorted slice
    // also SortFloats, SortStrings; options are Reverse
  SortMapInts(m map[string]int[, options]) []KVInt // return sorted map items
    // also SortMapFloats, SortMapStrings; options are Reverse, ByValue

Examples:
  # Say hello to the world
  ` + exampleHelloWorld + `

  # Print the average value of the last field
  ` + exampleAverage + `

  # Print 3rd field in milliseconds if record contains "GET" or "HEAD"
  ` + exampleMilliseconds + `

  # Print frequencies of unique words, most frequent first
  ` + exampleFrequencies

const (
	exampleHelloWorld   = `prig -b 'Println("Hello, world!")'`
	exampleAverage      = `prig -b 's := 0.0' 's += Float(F(NF()))' -e 'Println(s / float64(NR()))'`
	exampleMilliseconds = `prig 'if Match(` + "`" + `GET|HEAD` + "`" + `, F(0)) { Printf("%.0fms\n", Float(F(3))*1000) }'`
	exampleFrequencies  = `prig -b 'freqs := map[string]int{}' \
       'for i := 1; i <= NF(); i++ { freqs[strings.ToLower(F(i))]++ }' \
       -e 'for _, f := range SortMapInts(freqs, ByValue, Reverse) { ' \
       -e 'Println(f.K, f.V) }'`
)

var imports = map[string]struct{}{
	"bufio":   {},
	"fmt":     {},
	"os":      {},
	"regexp":  {},
	"sort":    {},
	"strconv": {},
	"strings": {},
}

type templateParams struct {
	FieldSep    string
	Imports     map[string]struct{}
	BeginID     string
	Begin       []string
	PerRecord   []string
	PerRecordID string
	End         []string
	EndID       string
}

var sourceTemplate = template.Must(template.New("source").Parse(`// Code generated by Prig (https://github.com/benhoyt/prig). DO NOT EDIT.

package main

import (
{{range $imp, $_ := .Imports}}
{{- printf "%q" $imp}}
{{end -}}
)

var (
	_output *bufio.Writer
	_record string
	_nr     int
    _fields []string
)

func main() {
	_output = bufio.NewWriter(os.Stdout)
	defer _output.Flush()

	// begin: {{.BeginID -}}
{{range .Begin}}
{{. -}}
{{end}}

{{if or .PerRecord .End}}
	_scanner := bufio.NewScanner(os.Stdin)
	for _scanner.Scan() {
		_record = _scanner.Text()
        _nr++
        _fields = nil

    // per-record: {{.PerRecordID -}}
{{range .PerRecord}}
{{. -}}
{{end}}
	}
	if _scanner.Err() != nil {
		_errorf("error reading stdin: %v", _scanner.Err())
	}
{{end}}

    // end: {{.EndID -}}
{{range .End}}
{{. -}}
{{end}}
}

func Print(args ...interface{}) {
	_, err := fmt.Fprint(_output, args...)
	if err != nil {
		_errorf("error writing output: %v", err)
	}
}

func Printf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(_output, format, args...)
	if err != nil {
		_errorf("error writing output: %v", err)
	}
}

func Println(args ...interface{}) {
	_, err := fmt.Fprintln(_output, args...)
	if err != nil {
		_errorf("error writing output: %v", err)
	}
}

func NR() int {
	return _nr
}

func F(i int) string {
	if i == 0 {
		return _record
	}
	_ensureFields()
    if i < 1 || i > len(_fields) {
        return ""
    }
    return _fields[i-1]
}

func Int(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func Float(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

var _fieldSepRegex *regexp.Regexp

func _ensureFields() {
	if _fields != nil {
		return
	}
{{if eq .FieldSep " "}}
	_fields = strings.Fields(_record)
{{else}}
	if _record == "" {
		_fields = []string{}
		return
	}
{{if le (len .FieldSep) 1}}
		_fields = strings.Split(_record, {{printf "%q" .FieldSep}})
{{else}}
		if _fieldSepRegex == nil {
			_fieldSepRegex = regexp.MustCompile({{printf "%q" .FieldSep}})
		}
		_fields = _fieldSepRegex.Split(_record, -1)
{{end}}
{{end}}
}

func NF() int {
	_ensureFields()
	return len(_fields)
}

func Match(re, s string) bool {
	regex := _reCompile(re)
	return regex.MatchString(s)
}

func Replace(re, s, repl string) string {
	regex := _reCompile(re)
	return regex.ReplaceAllString(s, repl)
}

func Submatches(re, s string) []string {
	regex := _reCompile(re)
	return regex.FindStringSubmatch(s)
}

var _reCache = make(map[string]*regexp.Regexp)

func _reCompile(re string) *regexp.Regexp {
	if regex, ok := _reCache[re]; ok {
		return regex
	}
	regex, err := regexp.Compile(re)
	if err != nil {
		_errorf("invalid regex %q: %v", re, err)
	}
	// Dumb, non-LRU cache: just cache the first 100 regexes
	if len(_reCache) < 100 {
		_reCache[re] = regex
	}
	return regex
}

func Substr(s string, n int, ms ...int) string {
	var m int
	switch len(ms) {
	case 0:
		m = len(s)
	case 1:
		m = ms[0]
	default:
		_errorf("Substr takes 2 or 3 arguments, not %d", len(ms)+2)
	}

	if n < 0 {
		n = len(s) + n
		if n < 0 {
			n = 0
		}
	}
	if n > len(s) {
		n = len(s)
	}

	if m < 0 {
		m = len(s) + m
		if m < 0 {
			m = 0
		}
	}
	if m > len(s) {
		m = len(s)
	}

	return s[n:m]
}

type _sortOption int

const (
	Reverse _sortOption = iota
	ByValue
)

func _getSortOptions(options []_sortOption, funcName string) (reverse bool) {
	for _, option := range options {
		switch option {
		case Reverse:
			reverse = true
		case ByValue:
			_errorf("invalid %s option ByValue", funcName)
		default:
			_errorf("invalid %s option %d", funcName, option)
		}
	}
	return reverse
}

func SortInts(s []int, options ..._sortOption) []int {
	reverse := _getSortOptions(options, "SortInts")
	cp := make([]int, len(s))
	copy(cp, s)
	sort.Ints(cp)
	if reverse {
		for i, j := 0, len(cp)-1; i < len(cp)/2; i, j = i+1, j-1 {
			tmp := cp[i]
			cp[i] = cp[j]
			cp[j] = tmp
		}
	}
	return cp
}

func SortFloats(s []float64, options ..._sortOption) []float64 {
	reverse := _getSortOptions(options, "SortFloats")
	cp := make([]float64, len(s))
	copy(cp, s)
	sort.Float64s(cp)
	if reverse {
		for i, j := 0, len(cp)-1; i < len(cp)/2; i, j = i+1, j-1 {
			tmp := cp[i]
			cp[i] = cp[j]
			cp[j] = tmp
		}
	}
	return cp
}

func SortStrings(s []string, options ..._sortOption) []string {
	reverse := _getSortOptions(options, "SortStrings")
	cp := make([]string, len(s))
	copy(cp, s)
	sort.Strings(cp)
	if reverse {
		for i, j := 0, len(cp)-1; i < len(cp)/2; i, j = i+1, j-1 {
			tmp := cp[i]
			cp[i] = cp[j]
			cp[j] = tmp
		}
	}
	return cp
}

type KVInt struct {
	K string
	V int
}

type KVFloat struct {
	K string
	V float64
}

type KVString struct {
	K string
	V string
}

func _getSortMapOptions(options []_sortOption, funcName string) (reverse, byValue bool) {
	for _, option := range options {
		switch option {
		case Reverse:
			reverse = true
		case ByValue:
			byValue = true
		default:
			_errorf("invalid %s option %d", funcName, option)
		}
	}
	return reverse, byValue
}

func SortMapInts(m map[string]int, options ..._sortOption) []KVInt {
	reverse, byValue := _getSortMapOptions(options, "SortMapInts")
	kvs := make([]KVInt, 0, len(m))
	for k, v := range m {
		kvs = append(kvs, KVInt{k, v})
	}
	if byValue {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].V == kvs[j].V {
				return kvs[i].K < kvs[j].K
			}
			return kvs[i].V < kvs[j].V
		})
	} else {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].K == kvs[j].K {
				return kvs[i].V < kvs[j].V
			}
			return kvs[i].K < kvs[j].K
		})
	}
	if reverse {
		for i, j := 0, len(kvs)-1; i < len(kvs)/2; i, j = i+1, j-1 {
			tmp := kvs[i]
			kvs[i] = kvs[j]
			kvs[j] = tmp
		}
	}
	return kvs
}

func SortMapFloats(m map[string]float64, options ..._sortOption) []KVFloat {
	reverse, byValue := _getSortMapOptions(options, "SortMapFloats")
	kvs := make([]KVFloat, 0, len(m))
	for k, v := range m {
		kvs = append(kvs, KVFloat{k, v})
	}
	if byValue {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].V == kvs[j].V {
				return kvs[i].K < kvs[j].K
			}
			return kvs[i].V < kvs[j].V
		})
	} else {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].K == kvs[j].K {
				return kvs[i].V < kvs[j].V
			}
			return kvs[i].K < kvs[j].K
		})
	}
	if reverse {
		for i, j := 0, len(kvs)-1; i < len(kvs)/2; i, j = i+1, j-1 {
			tmp := kvs[i]
			kvs[i] = kvs[j]
			kvs[j] = tmp
		}
	}
	return kvs
}

func SortMapStrings(m map[string]string, options ..._sortOption) []KVString {
	reverse, byValue := _getSortMapOptions(options, "SortMapStrings")
	kvs := make([]KVString, 0, len(m))
	for k, v := range m {
		kvs = append(kvs, KVString{k, v})
	}
	if byValue {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].V == kvs[j].V {
				return kvs[i].K < kvs[j].K
			}
			return kvs[i].V < kvs[j].V
		})
	} else {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].K == kvs[j].K {
				return kvs[i].V < kvs[j].V
			}
			return kvs[i].K < kvs[j].K
		})
	}
	if reverse {
		for i, j := 0, len(kvs)-1; i < len(kvs)/2; i, j = i+1, j-1 {
			tmp := kvs[i]
			kvs[i] = kvs[j]
			kvs[j] = tmp
		}
	}
	return kvs
}

func _errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
`))
