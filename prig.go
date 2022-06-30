// Prig is for Processing Records In Go. It's like AWK, but snobbish.
//
// It is based on a similar idea for Nim:
// https://github.com/c-blake/cligen/blob/master/examples/rp.nim
//
// Prig code is licensed under the MIT License.
//
// See README.md for more details.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	importspkg "golang.org/x/tools/imports"
)

const version = "v1.1.0"

var goVersionRegex = regexp.MustCompile(`^go version go1.(\d+)`)

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
	goExe := "go"

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
		case "-g":
			if i >= len(os.Args) {
				errorf("-g requires an argument")
			}
			goExe = os.Args[i]
			i++
		case "-i":
			imports[os.Args[i]] = struct{}{}
			if i >= len(os.Args) {
				errorf("-i requires an argument")
			}
			i++
		case "-h", "--help":
			fmt.Printf("%s\n", usage)
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

	if len(fieldSep) > 1 {
		_, err := regexp.Compile(fieldSep)
		if err != nil {
			errorf("invalid field separator: %v", err)
		}
	}

	// Use non-generic Sort/SortMap if importspkg.Process doesn't support
	// generics, or we're using a Go that doesn't support generics (<=1.17).
	sortFuncs := sortGeneric
	cmd := exec.Command(goExe, "version")
	output, err := cmd.CombinedOutput()
	if err == nil {
		matches := goVersionRegex.FindSubmatch(output)
		if matches != nil {
			goMinor, _ := strconv.Atoi(string(matches[1]))
			if goMinor <= 17 {
				sortFuncs = sortNonGeneric
			}
		}
	}
	_, err = importspkg.Process("", []byte("package x\nfunc f[T any]() {}"), nil)
	if err != nil {
		sortFuncs = sortNonGeneric
	}

	// Write source code to buffer
	var buffer bytes.Buffer
	params := &templateParams{
		FieldSep:  fieldSep,
		Imports:   imports,
		Begin:     begin,
		PerRecord: perRecord,
		End:       end,
		SortFuncs: sortFuncs,
	}
	err = sourceTemplate.Execute(&buffer, params)
	if err != nil {
		errorf("error executing template: %v", err)
	}
	bufferBytes := buffer.Bytes()

	// Add imports (also pretty-prints for printSource mode).
	sourceBytes, err := importspkg.Process("", bufferBytes, nil)
	if err != nil {
		parsed := parseErrors(err.Error(), string(bufferBytes), params)
		fmt.Fprint(os.Stderr, parsed)
		os.Exit(1)
	}
	if printSource {
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
	_, err = exec.LookPath(goExe)
	if err != nil {
		errorf("You must install Go to use 'prig', see https://go.dev/doc/install")
	}

	// Build the program with "go build"
	exeFilename := filepath.Join(tempDir, "main")
	if runtime.GOOS == "windows" {
		exeFilename += ".exe"
	}
	cmd = exec.Command(goExe, "build", "-o", exeFilename, goFilename)
	output, err = cmd.CombinedOutput()
	switch err.(type) {
	case nil:
	case *exec.ExitError:
		parsed := parseErrors(string(output), string(sourceBytes), params)
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

var compileErrorRe = regexp.MustCompile(`^(.*:)?(\d+):(\d+): (.*)`)

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
		lineNum, _ := strconv.Atoi(matches[2])
		colNum, _ := strconv.Atoi(matches[3])
		message := matches[4]
		sourceLine, caretLine := getSourceCaretLine(source, lineNum, colNum)
		fmt.Fprintf(&builder, "main.go:%d:%d: %s\n%s\n%s\n", lineNum, colNum, message, sourceLine, caretLine)
	}
	return builder.String()
}

func getSourceCaretLine(source string, line, col int) (sourceLine, caretLine string) {
	lines := strings.Split(source, "\n")
	if line < 1 || line > len(lines) {
		return "", ""
	}
	sourceLine = lines[line-1]
	numTabs := strings.Count(sourceLine[:col-1], "\t")
	runeColumn := utf8.RuneCountInString(sourceLine[:col-1])
	sourceLine = strings.Replace(sourceLine, "\t", "    ", -1)
	caretLine = strings.Repeat(" ", runeColumn) + strings.Repeat("   ", numTabs) + "^"
	return sourceLine, caretLine
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

const usage = `Prig ` + version + ` - Copyright (c) 2022 Ben Hoyt

Usage: prig [options] [-b 'begin code'] 'per-record code' [-e 'end code']

Prig is for Processing Records In Go. It's like AWK, but snobbish (Go! static
typing!). It runs 'begin code' first, then runs 'per-record code' for every
record (line) in the input, then runs 'end code'. Prig uses "go build", so it
requires the Go compiler: https://go.dev/doc/install

Options:
  -F char | re     field separator (single character or multi-char regex)
  -g executable    Go compiler to use (eg: "go1.18rc1", default "go")
  -h, --help       print help message and exit
  -i import        import Go package (normally automatic)
  -s               print formatted Go source instead of running
  -V, --version    print version number and exit

Built-in functions:
  F(i int) float64 // return field i as float64, int, or string
  I(i int) int     // (i==0 is entire record, i==1 is first field)
  S(i int) string

  NF() int // return number of fields in current record
  NR() int // return number of current record

  Print(args ...interface{})                 // fmt.Print, but buffered
  Printf(format string, args ...interface{}) // fmt.Printf, but buffered
  Println(args ...interface{})               // fmt.Println, but buffered

  Match(re, s string) bool            // report whether s contains match of re
  Replace(re, s, repl string) string  // replace all re matches in s with repl
  Submatches(re, s string) []string   // return slice of submatches of re in s
  Substr(s string, n[, m] int) string // s[n:m] but safe and allow negative n/m

  Sort[T int|float64|string](s []T) []T
    // return new sorted slice; also Sort(s, Reverse) to sort descending
  SortMap[T int|float64|string](m map[string]T) []KV[T]
    // return sorted slice of key-value pairs
    // also SortMap(s[, Reverse][, ByValue]) to sort descending or by value

Examples:
  # Run an arbitrary Go snippet; don't process input
  ` + exampleHelloWorld + `

  # Print the average value of the last field
  ` + exampleAverage + `

  # Print 3rd field in milliseconds if line contains "GET" or "HEAD"
  ` + exampleMilliseconds + `

  # Print frequencies of unique words, most frequent first
  ` + exampleFrequencies

// These are tested in prig_test.go to ensure we're testing our examples.
const (
	exampleHelloWorld   = `prig -b 'Println("Hello, world!", math.Pi)'`
	exampleAverage      = `prig -b 's := 0.0' 's += F(NF())' -e 'Println(s / float64(NR()))'`
	exampleMilliseconds = `prig 'if Match(` + "`" + `GET|HEAD` + "`" + `, S(0)) { Printf("%.0fms\n", F(3)*1000) }'`
	exampleFrequencies  = `prig -b 'freqs := map[string]int{}' \
       'for i := 1; i <= NF(); i++ { freqs[strings.ToLower(S(i))]++ }' \
       -e 'for _, f := range SortMap(freqs, ByValue, Reverse) { ' \
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
	FieldSep  string
	Imports   map[string]struct{}
	Begin     []string
	PerRecord []string
	End       []string
	SortFuncs string
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

{{range .Begin}}
{{. -}}
{{end}}

{{if or .PerRecord .End}}
	_scanner := bufio.NewScanner(os.Stdin)
	for _scanner.Scan() {
		_record = _scanner.Text()
        _nr++
        _fields = nil

{{range .PerRecord}}
{{. -}}
{{end}}
	}
	if _scanner.Err() != nil {
		_errorf("error reading stdin: %v", _scanner.Err())
	}
{{end}}

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

func S(i int) string {
	if i == 0 {
		return _record
	}
	_ensureFields()
    if i < 1 || i > len(_fields) {
        return ""
    }
    return _fields[i-1]
}

func I(i int) int {
	s := S(i)
	n, err := strconv.Atoi(s)
	if err != nil {
		f, _ := strconv.ParseFloat(s, 64)
		return int(f)
	}
	return n
}

func F(i int) float64 {
	s := S(i)
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
	matches := regex.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}
	return matches[1:]
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

	if n > m {
		return ""
	}

	return s[n:m]
}

type _sortOption int

const (
	Reverse _sortOption = iota
	ByValue
)

func _getSortOptions(options ..._sortOption) (reverse bool) {
	for _, option := range options {
		switch option {
		case Reverse:
			reverse = true
		case ByValue:
			_errorf("Sort option ByValue not valid")
		default:
			_errorf("Sort option %d valid", option)
		}
	}
	return reverse
}

func _getSortMapOptions(options ..._sortOption) (reverse, byValue bool) {
	for _, option := range options {
		switch option {
		case Reverse:
			reverse = true
		case ByValue:
			byValue = true
		default:
			_errorf("SortMap option %d not valid", option)
		}
	}
	return reverse, byValue
}

{{.SortFuncs}}

func _errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
`))

const sortGeneric = `
func Sort[T int|float64|string](s []T, options ..._sortOption) []T {
	reverse := _getSortOptions(options...)

	// TODO: probably could be improved when slices package arrives
	result := make([]T, len(s))
	copy(result, s)
	if reverse {
		sort.Slice(result, func(i, j int) bool {
			return result[i] > result[j]
		})
	} else {
		sort.Slice(result, func(i, j int) bool {
			return result[i] < result[j]
		})
	}
	return result
}

type KV[T int|float64|string] struct {
	K string
	V T
}

func SortMap[T int|float64|string](m map[string]T, options ..._sortOption) []KV[T] {
	reverse, byValue := _getSortMapOptions(options...)

	kvs := make([]KV[T], 0, len(m))
	for k, v := range m {
		kvs = append(kvs, KV[T]{k, v})
	}

	// TODO: probably could be improved when slices package arrives
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
`

const sortNonGeneric = `
func Sort(s interface{}, options ..._sortOption) []interface{} {
	reverse := _getSortOptions(options...)

	var result []interface{}
	switch s := s.(type) {
	case []int:
		cp := make([]int, len(s))
		copy(cp, s)
		sort.Ints(cp)
		result = make([]interface{}, len(s))
		for i, x := range cp {
			result[i] = x
		}
	case []float64:
		cp := make([]float64, len(s))
		copy(cp, s)
		sort.Float64s(cp)
		result = make([]interface{}, len(s))
		for i, x := range cp {
			result[i] = x
		}
	case []string:
		cp := make([]string, len(s))
		copy(cp, s)
		sort.Strings(cp)
		result = make([]interface{}, len(s))
		for i, x := range cp {
			result[i] = x
		}
	default:
		_errorf("Sort type must be int, float64, or string")
	}

	if reverse {
		for i, j := 0, len(result)-1; i < len(result)/2; i, j = i+1, j-1 {
			tmp := result[i]
			result[i] = result[j]
			result[j] = tmp
		}
	}
	return result
}

type KV struct {
	K string
	V interface{}
}

func SortMap(m interface{}, options ..._sortOption) []KV {
	reverse, byValue := _getSortMapOptions(options...)

	var kvs []KV
	var vLess func(i, j int) bool
	switch m := m.(type) {
	case map[string]int:
 		kvs = make([]KV, 0, len(m))
 		for k, v := range m {
			kvs = append(kvs, KV{k, v})
		}
		vLess = func(i, j int) bool {
			return kvs[i].V.(int) < kvs[j].V.(int)
		}
	case map[string]float64:
 		kvs = make([]KV, 0, len(m))
		for k, v := range m {
			kvs = append(kvs, KV{k, v})
		}
		vLess = func(i, j int) bool {
			return kvs[i].V.(float64) < kvs[j].V.(float64)
		}
	case map[string]string:
 		kvs = make([]KV, 0, len(m))
		for k, v := range m {
			kvs = append(kvs, KV{k, v})
		}
		vLess = func(i, j int) bool {
			return kvs[i].V.(string) < kvs[j].V.(string)
		}
	default:
		_errorf("SortMap values must be int, float64, or string")
	}

	if byValue {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].V == kvs[j].V {
				return kvs[i].K < kvs[j].K
			}
			return vLess(i, j)
		})
	} else {
		sort.Slice(kvs, func (i, j int) bool {
			if kvs[i].K == kvs[j].K {
				return vLess(i, j)
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
`
