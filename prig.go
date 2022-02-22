package main

/*
Based on a similar idea for Nim:
https://github.com/c-blake/cligen/blob/master/examples/rp.nim

TODO:
- Parse and prettify compile errors
- Add support for -F (changing field sep)
  + include single-byte sep and regex
- Add sub/gsub equivalent
- Add match equivalent, like Match(`regex`, s); remember RSTART, RLENGTH equivalents
  + also add shortcut for 'if Match(`regex`, R) { ... }' as '/regex/ { ... }'
- Add split equivalent
- Add substr equivalent
  + do we need it, or is s[n:m] enough? (doesn't handle out of bounds gracefully)
  + should indexes be 0-based or 1-based?
- Add system() equivalent?
- Add sort helpers to sort slices and map keys/values
- Are S() I() F() the right names? Should it be F() FI() FF() instead?
- Should we add helpers like MapI() -> make(map[string]int)?
- Is NF() too awkward as a function instead of a var NF? With a var we couldn't have lazy splitting.


prig -b 'c:=map[string]int{}' 'for i:=1; i<=NF(); i++ { c[Lower(S(i))]++ }' -e 'for k, v := range c { Println(k, v) }'
awk '{ for (i=1; i<=NF; i++) c[tolower($i)]++ } END { for (k in c) { print k, c[k] } }'

*/

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

func main() {
	// Parse command line arguments
	if len(os.Args) <= 1 {
		errorf(usage)
	}
	var begin []string
	var perRecord []string
	var end []string
	for i := 1; i < len(os.Args); {
		arg := os.Args[i]
		i++

		switch arg {
		case "-b", "--begin":
			if i >= len(os.Args) {
				errorf("-b requires an argument")
			}
			begin = append(begin, os.Args[i])
			i++
		case "-e", "--end":
			if i >= len(os.Args) {
				errorf("-e requires an argument")
			}
			end = append(end, os.Args[i])
			i++
		case "-i", "--import":
			imports[os.Args[i]] = struct{}{}
			if i >= len(os.Args) {
				errorf("-e requires an argument")
			}
			i++
		case "-h", "--help":
			fmt.Println(usage)
			return
		default:
			perRecord = append(perRecord, arg)
		}
	}

	// Create a temporary work directory and .go file
	tempDir, err := os.MkdirTemp("", "prig_")
	if err != nil {
		errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	goFilename := filepath.Join(tempDir, "main.go")
	source, err := os.Create(goFilename)
	if err != nil {
		errorf("error creating temp file: %v", err)
	}

	// Write source code to .go file
	err = sourceTemplate.Execute(source, &templateParams{
		Imports:   imports,
		Begin:     begin,
		PerRecord: perRecord,
		End:       end,
	})
	if err != nil {
		errorf("error executing template: %v", err)
	}
	err = source.Close()
	if err != nil {
		errorf("error closing temp file: %v", err)
	}

	// Build it with "go build"
	exeFilename := filepath.Join(tempDir, "main")
	cmd := exec.Command("go", "build", "-o", exeFilename, goFilename)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// TODO: parse and prettify compile errors?
		b, _ := os.ReadFile(goFilename)
		fmt.Fprint(os.Stderr, string(b), "\n", string(output))
		os.Exit(1)
	}

	// Then run the executable we just built
	cmd = exec.Command(exeFilename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		exitCode := cmd.ProcessState.ExitCode()
		if exitCode != -1 {
			os.Exit(exitCode)
		}
		errorf("error running program: %v", err)
	}
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

const usage = "usage: prig [-b 'begin code'] 'per-record code' [-e 'end code']"

var imports = map[string]struct{}{
	"bufio":   {},
	"fmt":     {},
	"os":      {},
	"strings": {},
}

type templateParams struct {
	Imports   map[string]struct{}
	Begin     []string
	PerRecord []string
	End       []string
}

var sourceTemplate = template.Must(template.New("source").Parse(`
package main

import (
{{range $imp, $_ := .Imports}}
{{printf "%q" $imp}}
{{end}}
)

var (
	_output *bufio.Writer

    R  string
    NR int

    _fields []string
)

func main() {
	_output = bufio.NewWriter(os.Stdout)
	defer _output.Flush()

{{range .Begin}}
{{.}}
{{end}}

{{if .PerRecord}}
	_scanner := bufio.NewScanner(os.Stdin)
	for _scanner.Scan() {
		R = _scanner.Text()
        NR++
        _fields = nil

{{range .PerRecord}}
{{.}}
{{end}}
	}
	if _scanner.Err() != nil {
		_errorf("error reading stdin: %v", _scanner.Err())
	}
{{end}}

{{range .End}}
{{.}}
{{end}}
}

func Lower(s string) string {
	return strings.ToLower(s)
}

func Upper(s string) string {
	return strings.ToUpper(s)
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

func Sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

func S(n int) string {
	if n == 0 {
		return R
	}
	_ensureFields()
    if n < 1 || n > len(_fields) {
        return ""
    }
    return _fields[n-1]
}

func _ensureFields() {
	if _fields != nil {
		return
	}
	_fields = strings.Fields(R)
}

func NF() int {
	_ensureFields()
	return len(_fields)
}

func _errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
`))
