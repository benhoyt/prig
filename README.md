
# Prig: the snobbish AWK

Prig is for Processing Records In Go. It's like AWK, but snobbish (Go! static typing!). It's also faster, and if you know Go, you don't need to learn AWK.

You can also read my [**article on why and how I wrote Prig**](https://benhoyt.com/writings/prig/). The tl;dr is "no good reason". :-)


## How to use Prig

To install `prig`, type `go install github.com/benhoyt/prig@latest` (TODO: test). Prig itself runs the code using `go build`, so even if you have a `prig` binary it requires the Go compiler to be [installed](https://go.dev/doc/install).

As a simple example, you can try this script which prints a modified version of the second field for each line of input (the full URL in this example):

```
$ cat logs.txt
GET /robots.txt HTTP/1.1
HEAD /README.md HTTP/1.1
GET /wp-admin/ HTTP/1.0
$ prig 'Println("https://example.com" + S(2))' <logs.txt
https://example.com/robots.txt
https://example.com/README.md
https://example.com/wp-admin/
```

To get help, run `prig` without any arguments or with the `-h` argument. Help output is copied below:

```
Prig v1.0.0 - Copyright (c) 2022 Ben Hoyt

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

  Print(args ...any)                 // fmt.Print, but buffered
  Printf(format string, args ...any) // fmt.Printf, but buffered
  Println(args ...any)               // fmt.Println, but buffered

  Match(re, s string) bool            // report whether s contains match of re
  Replace(re, s, repl string) string  // replace all re matches in s with repl
  Submatches(re, s string) []string   // return slice of submatches of re in s
  Substr(s string, n[, m] int) string // s[n:m] but safe and allow negative n/m

  Sort[T int|float64|string](s []T) []T
    // return new sorted slice; also Sort(s, Reverse) to sort descending
  SortMap[T int|float64|string](m map[string]T) []KV[T]
    // return sorted slice of key-value pairs
    // also Sort(s[, Reverse][, ByValue]) to sort descending or by value

Examples:
  # Run an arbitrary Go snippet; don't process input
  prig -b 'Println("Hello, world!", math.Pi)'

  # Print the average value of the last field
  prig -b 's := 0.0' 's += F(NF())' -e 'Println(s / float64(NR()))'

  # Print 3rd field in milliseconds if line contains "GET" or "HEAD"
  prig 'if Match(`GET|HEAD`, S(0)) { Printf("%.0fms\n", F(3)*1000) }'

  # Print frequencies of unique words, most frequent first
  prig -b 'freqs := map[string]int{}' \
       'for i := 1; i <= NF(); i++ { freqs[strings.ToLower(S(i))]++ }' \
       -e 'for _, f := range SortMap(freqs, ByValue, Reverse) { ' \
       -e 'Println(f.K, f.V) }'
```

Prig uses the [golang.org/x/tools/imports](https://golang.org/x/tools/imports) package, so imports are usually automatic (use `-i` if you need to override). And that's really all you need to know -- all the code is pure, ordinary Go.


## Other info

If you want a real, POSIX-compatible version of AWK for use in Go programs, see my [GoAWK](https://github.com/benhoyt/goawk) project.

Prig was based on [rp](https://github.com/c-blake/cligen/blob/master/examples/rp.nim), a similar idea for Nim written by TODO.

Prig is licensed under an open source [MIT license](https://github.com/benhoyt/prig/blob/master/LICENSE.txt).
