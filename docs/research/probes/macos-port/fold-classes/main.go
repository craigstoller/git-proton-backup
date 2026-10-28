//go:build ignore

// orbits: print every unicode.SimpleFold orbit with more than one member, one
// orbit per line as space-separated hex code points, smallest first.
package main

import (
	"fmt"
	"sort"
	"unicode"
)

func main() {
	seen := map[rune]bool{}
	fmt.Println("# unicode.Version", unicode.Version)
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if seen[r] || unicode.SimpleFold(r) == r {
			continue
		}
		orbit := []rune{r}
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			orbit = append(orbit, f)
		}
		sort.Slice(orbit, func(i, j int) bool { return orbit[i] < orbit[j] })
		for _, m := range orbit {
			seen[m] = true
			fmt.Printf("%X ", m)
		}
		fmt.Println()
	}
}
