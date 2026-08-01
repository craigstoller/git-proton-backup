package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	// git passes: git-remote-proton <remote-name> <url>
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "git-remote-proton: must be run by git as a remote helper")
		os.Exit(1)
	}
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for in.Scan() {
		switch line := in.Text(); line {
		case "capabilities":
			// Only what Stage 2 implements. `option` is advertised because the
			// unsupported-option state machine depends on receiving them.
			fmt.Fprint(out, "option\npush\n\n")
			out.Flush()
		case "":
			return
		default:
			fmt.Fprintf(os.Stderr, "unsupported command: %q\n", line)
			os.Exit(1)
		}
	}
}
