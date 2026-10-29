//go:build ignore

package main

import (
	"fmt"
	"unicode"
)

func minFold(r rune) rune {
	m := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < m {
			m = f
		}
	}
	return m
}

func main() {
	for _, r := range []rune{0x03B9, 0x0399, 0x1FBE, 0x0345} {
		m := minFold(r)
		fmt.Printf("U+%04X -> U+%04X (combining mark: %v)\n", r, m, unicode.Is(unicode.Mn, m))
	}
}
