package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TODO: add TestMain to build ./prig

type test struct {
	name string
	args []string
	in   string
	out  string
	err  string
}

var prigTests = []test{
	{
		name: "single begin",
		args: []string{`-b`, `Println("Hello, world")`},
		out:  "Hello, world\n",
	},
}

func TestPrig(t *testing.T) {
	runTests(t, prigTests)
}

func runTests(t *testing.T, tests []test) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("./prig", test.args...)
			cmd.Stdin = strings.NewReader(test.in)
			outputBytes, err := cmd.CombinedOutput()
			output := string(outputBytes)
			if err != nil {
				if test.err == "" {
					t.Fatalf("expected success, got error:\n%s", output)
				}
				if output != test.err {
					t.Fatalf("expected first error, got second:\n%s\n-----\n%s", test.err, output)
				}
				return
			}
			if test.err != "" {
				t.Fatalf("got success, expected error:\n%s", test.err)
			}

			if output != test.out {
				t.Fatalf("expected first output, got second:\n%s\n-----\n%s", test.out, output)
			}
		})
	}
}

func TestExamples(t *testing.T) {
	tests := []test{
		{
			name: "HelloWorld",
			args: exampleToArgs(t, exampleHelloWorld),
			out:  "Hello, world!\n",
		},
		{
			name: "Average",
			args: exampleToArgs(t, exampleAverage),
			in:   "a b 400\nc d 200\ne f 200\ng h 200",
			out:  "250\n",
		},
		{
			name: "Milliseconds",
			args: exampleToArgs(t, exampleMilliseconds),
			in:   "1 GET 3.14159\n2 HEAD 4.0\n3 GET 1.0\n4 GET 100.23\n",
			out:  "3142ms\n4000ms\n1000ms\n100230ms\n",
		},
		{
			name: "Frequencies",
			args: exampleToArgs(t, exampleFrequencies),
			in:   "The foo bar foo bar\nthe the the\nend.\n",
			out:  "the 4\nfoo 2\nbar 2\nend. 1\n",
		},
	}
	runTests(t, tests)
}

func exampleToArgs(t *testing.T, s string) []string {
	t.Helper()
	if !strings.HasPrefix(s, "prig ") {
		t.Fatal(`example must start with "prig "`)
	}
	s = s[5:]
	s = strings.ReplaceAll(s, "\\\n", "")
	parts := strings.Split(s, "'")
	var args []string
	for i, part := range parts {
		if i%2 == 0 {
			args = append(args, strings.Fields(part)...)
		} else {
			args = append(args, part)
		}
	}
	return args
}
