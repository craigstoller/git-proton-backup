//go:build ignore

package main

import "fmt"

func main() {
	nfc, nfd := "refs/heads/café", "refs/heads/café"
	fmt.Printf("%%q:  %q  %q  equal=%v\n", nfc, nfd, fmt.Sprintf("%q", nfc) == fmt.Sprintf("%q", nfd))
	fmt.Printf("%%+q: %+q  %+q\n", nfc, nfd)
}
