

There's a reason for AWK - code is shorter and less punc noise.
But compare speed!

```
Examples:
  # Say hello to the world
  prig -b 'Println("Hello, world!")'
  awk 'BEGIN { print "Hello, world!" }'

  # Print the average value of the last field
  prig -b 's := 0.0' 's += Float(F(NF()))' -e 'Println(s / float64(NR()))'
  awk '{ s += $NF } END { print s/NR }'

  # Print 3rd field in milliseconds if record contains "GET" or "HEAD"
  prig 'if Match(`GET|HEAD`, F(0)) { Printf("%.0fms\n", Float(F(3))*1000) }'
  awk '/GET|HEAD/ { printf "%.0fms", $3*1000 }'

  # Print frequencies of unique words, most frequent first
  prig -b 'freqs := map[string]int{}' \
       'for i := 1; i <= NF(); i++ { freqs[strings.ToLower(F(i))]++ }' \
       -e 'for _, f := range SortMap(freqs, ByValue, Reverse) { ' \
       -e 'Println(f.K, f.V) }'
  awk '{ for (i = 1; i <= NF; i++) freqs[lower($i)]++ }'
      'END { for (k in freqs) print k, freqs[k] | "sort -nr"'
