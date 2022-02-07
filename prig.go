package main

// Based on a similar idea for Nim:
// https://github.com/c-blake/cligen/blob/master/examples/rp.nim

// TODO: grip is not a great name (search and replace "grip" with something else)
// - maybe "pogo" (that's used by a couple other very different Go projects)
// - maybe prig: the snobbish Processor of Records In Go

// - NF being a variable means we have to eagerly split fields
// - make it NF() instead?

/*
grip -b 'counts:=MapI()' 'for i:=1; i<=NF; i++ { counts[strings.ToLower(S(i))]++ }' -e 'for k, v := range counts { fmt.Println(k, v) }'
*/

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
		case "-h", "--help":
			fmt.Println(usage)
			return
		default:
			perRecord = append(perRecord, arg)
		}
	}

	// Write source code to buffer
	// TODO: write directly to temp file?
	var source bytes.Buffer
	source.WriteString(header)
	for _, code := range begin {
		source.WriteString(code)
		source.WriteString("\n")
	}
	if len(perRecord) > 0 {
		source.WriteString(scanStart)
		for _, code := range perRecord {
			source.WriteString(code)
			source.WriteString("\n")
		}
		source.WriteString(scanEnd)
	}
	for _, code := range end {
		source.WriteString(code)
		source.WriteString("\n")
	}
	source.WriteString(footer)

	// Write source to a temporary file (and delete it afterwards)
	file, err := os.CreateTemp("", "grip_*.go")
	if err != nil {
		errorf("error creating temp file: %v", err)
	}
	defer os.Remove(file.Name())
	_, err = file.Write(source.Bytes())
	if err != nil {
		errorf("error writing temp file: %v", err)
	}
	err = file.Close()
	if err != nil {
		errorf("error closing temp file: %v", err)
	}

	// Then use "go run" to run it
	cmd := exec.Command("go", "run", file.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		// TODO: handle "go run" errors better, distingiush between compile and run error
		errorf("error compiling or running program: %v", err)
	}
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

const usage = "usage: grip [-b 'begin code'] 'per-record code' [-e 'end code']"

const header = `
package main

import (
    "bufio"
    "fmt"
    "os"
    "strings"
)

var (
    _ = bufio.NewScanner
    _ = fmt.Fprintln
    _ = os.Exit
    _ = strings.Fields
)

var (
    R string
    NR int
    _fields []string
    NF int
)

func main() {
`

const scanStart = `
	_scanner := bufio.NewScanner(os.Stdin)
	for _scanner.Scan() {
		R = _scanner.Text()
        NR++
		_fields = strings.Fields(R)
		NF = len(_fields)
`

const scanEnd = `
	}
	if _scanner.Err() != nil {
		fmt.Fprintln(os.Stderr, "error reading stdin:", _scanner.Err())
		os.Exit(1)
	}
`

const footer = `
}

func S(n int) string {
    if n < 1 || n > len(_fields) {
        return ""
    }
    return _fields[n-1]
}

func MapI() map[string]int {
    return make(map[string]int)
}
`
