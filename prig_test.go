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
	{
		name: "per-record with if clause",
		args: []string{`if Match("foo", F(0)) { Println(F(1)) }`},
		in:   "foo a\nbar b\n",
		out:  "foo\n",
	},
	{
		name: "multiple begin, per-record, end",
		args: []string{`-b`, `x:=1`, `-b`, `y:=2`, `Println(F(1))`, `-e`, `Println(x)`, `-e`, `Println(y)`},
		in:   "foo a\nbar b\n",
		out:  "foo\nbar\n1\n2\n",
	},
	{
		name: "Print()",
		args: []string{`-b`, `Print("foo", "bar")`},
		out:  "foobar",
	},
	{
		name: "Printf()",
		args: []string{`-b`, `Printf("%d %.03f %s", 42, 1.2345, "foo bar")`},
		out:  "42 1.234 foo bar",
	},
	{
		name: "Println()",
		args: []string{`-b`, `Println(); Println(42, "foo")`},
		out:  "\n42 foo\n",
	},
	{
		name: "NR() and F(0)",
		args: []string{`Println(NR(), F(0))`},
		in:   "foo bar\nbazz\nbuzz",
		out:  "1 foo bar\n2 bazz\n3 buzz\n",
	},
	{
		name: "F(n)",
		args: []string{`Println(F(1), F(3), F(5), F(-1))`},
		in:   "aye b cee\n1 2 3.0\n",
		out:  "aye cee  \n1 3.0  \n",
	},
	{
		name: "Int()",
		args: []string{`var x, y int; x, y = Int(F(1)), Int(F(2)); Println(x, y)`},
		in:   "1 2.9\n-48 0\nfoo bar\n",
		out:  "1 2\n-48 0\n0 0\n",
	},
	{
		name: "NF()",
		args: []string{`for i:=1; i<=NF(); i++ { Println(NR(), i, F(i)) }`},
		in:   "aye b cee\n1 2 3\n",
		out:  "1 1 aye\n1 2 b\n1 3 cee\n2 1 1\n2 2 2\n2 3 3\n",
	},
	{
		name: "Match()",
		args: []string{`Println(Match("GET|HEAD", F(0)))`},
		in:   "GET a\nHEAD b\nPOST c\nd GET\nget e\nHEADY 4\n",
		out:  "true\ntrue\nfalse\ntrue\nfalse\ntrue\n",
	},
	{
		name: "Replace()",
		args: []string{`Println(Replace("[st]he", F(0), "THE"))`},
		in:   "the The the\nxthe shee\nfoo bar",
		out:  "THE The THE\nxTHE THEe\nfoo bar\n",
	},
	{
		name: "Submatches() without anchors",
		args: []string{"Println(Submatches(`/user/(.+)/(\\d+)`, F(1)))"},
		in:   "\nfoo\n/v1/user/benhoyt/42/\n/user/xyz/100\n",
		out:  "[]\n[]\n[benhoyt 42]\n[xyz 100]\n",
	},
	{
		name: "Submatches() without anchors",
		args: []string{"Println(Submatches(`^/user/(.+)/(\\d+)$`, F(1)))"},
		in:   "\nfoo\n/v1/user/benhoyt/42/\n/user/xyz/100\n",
		out:  "[]\n[]\n[]\n[xyz 100]\n",
	},
	{
		name: "Substr()",
		args: []string{
			`-b`, `Println(Substr("foobar", 0))`,
			`-b`, `Println(Substr("foobar", 1))`,
			`-b`, `Println(Substr("foobar", 5))`,
			`-b`, `Println(Substr("foobar", 6))`,
			`-b`, `Println(Substr("foobar", 7))`,
			`-b`, `Println(Substr("foobar", -1))`,
			`-b`, `Println(Substr("foobar", -5))`,
			`-b`, `Println(Substr("foobar", -6))`,
			`-b`, `Println(Substr("foobar", -7))`,
			`-b`, `Println(Substr("prig", 0, 4))`,
			`-b`, `Println(Substr("prig", 1, 7))`,
			`-b`, `Println(Substr("prig", 1, 3))`,
			`-b`, `Println(Substr("prig", 2, 2))`,
			`-b`, `Println(Substr("prig", 3, 1))`,
			`-b`, `Println(Substr("prig", 1, -1))`,
			`-b`, `Println(Substr("prig", 0, -3))`,
			`-b`, `Println(Substr("prig", 0, -7))`,
		},
		out: `
foobar
oobar
r


r
oobar
foobar
foobar
prig
rig
ri


ri
p

`[1:],
	},
	{
		name: "Sort() ints",
		args: []string{
			`-b`, `s := []int{}; Println(s, Sort(s))`,
			`-b`, `s = []int{3, 2, 1, 2}; Println(s, Sort(s))`,
			`-b`, `s = []int{3, 0, 2, 1, -2}; Println(s, Sort(s, Reverse))`,
		},
		out: "[] []\n[3 2 1 2] [1 2 2 3]\n[3 0 2 1 -2] [3 2 1 0 -2]\n",
	},
	{
		name: "Sort() floats",
		args: []string{
			`-b`, `s := []float64{}; Println(s, Sort(s))`,
			`-b`, `s = []float64{3.0, 3, 3.14, 0, -2e3}; Println(s, Sort(s))`,
			`-b`, `s = []float64{3.0, 3, 3.14, 0, -2e3}; Println(s, Sort(s, Reverse))`,
		},
		out: "[] []\n[3 3 3.14 0 -2000] [-2000 0 3 3 3.14]\n[3 3 3.14 0 -2000] [3.14 3 3 0 -2000]\n",
	},
	{
		name: "Sort() strings",
		args: []string{
			`-b`, `s := []string{}; Println(s, Sort(s))`,
			`-b`, `s = []string{"B", "b", "A", "x"}; Println(s, Sort(s))`,
			`-b`, `s = []string{"B", "b", "A", "x"}; Println(s, Sort(s, Reverse))`,
		},
		out: "[] []\n[B b A x] [A B b x]\n[B b A x] [x b B A]\n",
	},
	{
		name: "Sort() invalid type",
		args: []string{`-b`, `Println(Sort(42))`},
		err:  "Sort type must be int, float64, or string\n",
	},
	{
		name: "Sort() invalid option",
		args: []string{`-b`, `Println(Sort([]int{4, 2}, ByValue))`},
		err:  "Sort option ByValue not valid\n",
	},
	{
		name: "SortMap() ints",
		args: []string{
			`-b`, `Println(SortMap(map[string]int{}))`,
			`-b`, `kvs := SortMap(map[string]int{"a": 2, "b": 1}); for _, kv := range kvs { Println(kv.K, kv.V) }`,
			`-b`, `Println(SortMap(map[string]int{"a": 2, "b": 1, "c": 0, "d": 1}))`,
			`-b`, `Println(SortMap(map[string]int{"a": 2, "b": 1, "c": 0, "d": 1}, ByValue))`,
			`-b`, `Println(SortMap(map[string]int{"a": 2, "b": 1, "c": 0, "d": 1}, Reverse))`,
			`-b`, `Println(SortMap(map[string]int{"a": 2, "b": 1, "c": 0, "d": 1}, ByValue, Reverse))`,
		},
		out: `
[]
a 2
b 1
[{a 2} {b 1} {c 0} {d 1}]
[{c 0} {b 1} {d 1} {a 2}]
[{d 1} {c 0} {b 1} {a 2}]
[{a 2} {d 1} {b 1} {c 0}]
`[1:],
	},
	{
		name: "SortMap() floats",
		args: []string{
			`-b`, `Println(SortMap(map[string]float64{}))`,
			`-b`, `kvs := SortMap(map[string]float64{"a": 3.14, "b": 1}); for _, kv := range kvs { Println(kv.K, kv.V) }`,
			`-b`, `Println(SortMap(map[string]float64{"a": 3.14, "b": 1, "c": 0, "d": 1}))`,
			`-b`, `Println(SortMap(map[string]float64{"a": 3.14, "b": 1, "c": 0, "d": 1}, ByValue))`,
			`-b`, `Println(SortMap(map[string]float64{"a": 3.14, "b": 1, "c": 0, "d": 1}, Reverse))`,
			`-b`, `Println(SortMap(map[string]float64{"a": 3.14, "b": 1, "c": 0, "d": 1}, ByValue, Reverse))`,
		},
		out: `
[]
a 3.14
b 1
[{a 3.14} {b 1} {c 0} {d 1}]
[{c 0} {b 1} {d 1} {a 3.14}]
[{d 1} {c 0} {b 1} {a 3.14}]
[{a 3.14} {d 1} {b 1} {c 0}]
`[1:],
	},
	{
		name: "SortMap() strings",
		args: []string{
			`-b`, `Println(SortMap(map[string]string{}))`,
			`-b`, `kvs := SortMap(map[string]string{"a": "2", "b": "1"}); for _, kv := range kvs { Println(kv.K, kv.V) }`,
			`-b`, `Println(SortMap(map[string]string{"a": "2", "b": "1", "c": "0", "d": "1"}))`,
			`-b`, `Println(SortMap(map[string]string{"a": "2", "b": "1", "c": "0", "d": "1"}, ByValue))`,
			`-b`, `Println(SortMap(map[string]string{"a": "2", "b": "1", "c": "0", "d": "1"}, Reverse))`,
			`-b`, `Println(SortMap(map[string]string{"a": "2", "b": "1", "c": "0", "d": "1"}, ByValue, Reverse))`,
		},
		out: `
[]
a 2
b 1
[{a 2} {b 1} {c 0} {d 1}]
[{c 0} {b 1} {d 1} {a 2}]
[{d 1} {c 0} {b 1} {a 2}]
[{a 2} {d 1} {b 1} {c 0}]
`[1:],
	},
	{
		name: "SortMap() invalid type",
		args: []string{`-b`, `Println(SortMap(map[string]uint{}))`},
		err:  "SortMap values must be int, float64, or string\n",
	},
	{
		name: "SortMap() invalid option",
		args: []string{`-b`, `Println(SortMap(map[string]int{"a": 1}, 42))`},
		err:  "SortMap option 42 not valid\n",
	},
	{
		name: "default field separator",
		args: []string{`Printf("%v,%v,%v\n", F(1), F(2), F(3))`},
		in:   "a b c\n  one  two\t\tthree \nxx  yy",
		out:  "a,b,c\none,two,three\nxx,yy,\n",
	},
	{
		name: "one-character field separator -F<sep>",
		args: []string{`-F,`, `Printf("%v.%v.%v\n", F(1), F(2), F(3))`},
		in:   "a,b,c\n  one,two  ,three\nxx,yy",
		out:  "a.b.c\n  one.two  .three\nxx.yy.\n",
	},
	{
		name: "one-character field separator -F <sep>",
		args: []string{`-F`, `,`, `Printf("%v.%v.%v\n", F(1), F(2), F(3))`},
		in:   "a,b,c\n  one,two  ,three\nxx,yy",
		out:  "a.b.c\n  one.two  .three\nxx.yy.\n",
	},
	{
		name: "regex field separator",
		args: []string{`-F[.,]`, `Printf("%v.%v.%v\n", F(1), F(2), F(3))`},
		in:   "a,b.c\n  one.two  ,three\nxx,yy",
		out:  "a.b.c\n  one.two  .three\nxx.yy.\n",
	},
	{
		name: "regex field separator error",
		args: []string{`-F[.,`, `Println()`},
		err:  "invalid field separator: error parsing regexp: missing closing ]: `[.,`\n",
	},
	{
		name: "compile errors",
		args: []string{`-b`, `B`, `@`, `-e`, `E`},
		err: `
begin1:1:1: undefined: B
B
^
perrecord1:1:1: invalid character U+0040 '@'
@
^
end1:1:1: undefined: E
E
^
`[1:],
	},
	{
		name: "compile error line number - begin",
		args: []string{`-b`, "if true {\n    foo\n}"},
		err: `
begin1:2:5: undefined: foo
    foo
    ^
`[1:],
	},
	{
		name: "compile error line number - per-record",
		args: []string{"if true {\n    foo\n}"},
		err: `
perrecord1:2:5: undefined: foo
    foo
    ^
`[1:],
	},
	{
		name: "compile error line number - end",
		args: []string{`-e`, "if true {\n    foo\n}"},
		err: `
end1:2:5: undefined: foo
    foo
    ^
`[1:],
	},
	{
		name: "version -V",
		args: []string{`-V`},
		out:  version + "\n",
	},
	{
		name: "version --version",
		args: []string{`--version`},
		out:  version + "\n",
	},
}

func TestPrig(t *testing.T) {
	runTests(t, prigTests)
}

func runTests(t *testing.T, tests []test) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := strings.NewReader(test.in)
			cmd := exec.Command("./prig", test.args...)
			cmd.Stdin = in
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
