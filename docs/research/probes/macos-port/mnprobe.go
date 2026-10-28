//go:build ignore

// mnprobe: would a stdlib-only rule "refuse a ref name containing a combining
// mark (unicode.Mn)" refuse ordinary, already-NFC names? Each sample is a
// common word written as Go escapes so this file stays ASCII.
package main

import (
	"fmt"
	"unicode"
)

func main() {
	samples := []struct{ label, s string }{
		{"cafe, NFC (precomposed e-acute)", "café"},
		{"cafe, NFD (e + combining acute)", "café"},
		{"Vietnamese 'Viet', NFC", "Việt"},
		{"Hindi 'hindi' (with anusvara)", "हिंदी"},
		{"Hindi 'hindi' (with virama)", "हिन्दी"},
		{"Thai 'thai'", "ไทย"},
		{"Thai 'language' (phasa)", "ภาษา"},
		{"Thai 'English' (angkrit)", "อังกฤษ"},
		{"Korean 'hangul', NFC", "한글"},
		{"Japanese 'branch' (kanji)", "枝"},
		{"Japanese 'ga', NFC (precomposed)", "が"},
		{"Japanese 'ga', NFD (ka + dakuten)", "が"},
	}
	for _, x := range samples {
		var marks []string
		for _, r := range x.s {
			if unicode.Is(unicode.Mn, r) {
				marks = append(marks, fmt.Sprintf("U+%04X", r))
			}
		}
		verdict := "allowed"
		if len(marks) > 0 {
			verdict = "REFUSED, combining marks " + fmt.Sprint(marks)
		}
		fmt.Printf("%-36s %s\n", x.label, verdict)
	}
}
