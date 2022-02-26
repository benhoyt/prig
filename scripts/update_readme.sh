#!/bin/sh

set -e

go build

./prig -h >help.txt

./prig 'if Match("^Prig v1", S(0)) { return }' \
       'Println(S(0))' \
       <README.md >head.txt

./prig -b 'start, end := false, false' \
       'if Match("^Prig v1", S(0)) { start = true }' \
       'if start && Match("^```", S(0)) { end = true }' \
       'if end { Println(S(0)) }' \
       <README.md >tail.txt

cat head.txt help.txt tail.txt >README.md

rm help.txt head.txt tail.txt
