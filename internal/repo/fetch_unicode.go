//go:build ignore

// fetch_unicode downloads the three pinned Unicode input files into
// unicodedata/<version>/ over HTTPS. It is the only code in the repository that
// touches the network, and it is run by hand, from internal/repo, when the pin
// moves:
//
//	go run fetch_unicode.go -version <v>
//
// Unicode publishes no checksums for these files (checked 2026-10-27: none in
// the 17.0.0 ucd/ listing or either ReadMe), so the command cannot verify them
// against a published value. It does what can be done (Amendment MA2):
//
//   - it requires CaseFolding.txt and NormalizationTest.txt to open with their
//     own version line, "# CaseFolding-<v>.txt" and "# NormalizationTest-<v>.txt";
//     UnicodeData.txt has no version line of its own;
//   - it prints each file's size and SHA-256, for a reviewer to compare with a
//     second fetch from another network and to write into the plan.
//
// Nothing is written unless all three downloads pass. A file that already
// exists with different bytes is never overwritten: a committed pin that a
// fresh fetch disagrees with is a finding to investigate, so the command stops
// and names both hashes. After a good fetch: put the new -version in the
// go:generate directive in canon.go, run go generate, update the checksums in
// canon_test.go, and delete the old unicodedata directory. canon_test.go's
// pinned constants are pinnedUnicodeInputs' sizes and SHA-256s and the version
// its test requires: update all of them, from the sizes and hashes printed here.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	baseURL = "https://www.unicode.org/Public"

	// maxBytes bounds one download. NormalizationTest.txt, the largest, is
	// under 3 MB.
	maxBytes = 16 << 20
)

var files = []string{"UnicodeData.txt", "CaseFolding.txt", "NormalizationTest.txt"}

func main() {
	fs := flag.NewFlagSet("fetch_unicode", flag.ExitOnError)
	version := fs.String("version", "", "Unicode version to fetch, such as 17.0.0 (required)")
	fs.Parse(os.Args[1:])
	if !validVersion(*version) {
		fmt.Fprintf(os.Stderr, "fetch_unicode: -version must look like 17.0.0, got %q\n", *version)
		os.Exit(2)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	if err := fetch(client, baseURL, *version, filepath.Join("unicodedata", *version), os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "fetch_unicode: %v\n", err)
		os.Exit(1)
	}
}

// validVersion accepts digits separated by single dots; the version goes into
// a URL and a directory name.
func validVersion(v string) bool {
	if v == "" || v[0] == '.' || v[len(v)-1] == '.' || strings.Contains(v, "..") {
		return false
	}
	for i := 0; i < len(v); i++ {
		if c := v[i]; c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func fetch(client *http.Client, base, version, dir string, stdout io.Writer) error {
	if !strings.HasPrefix(base, "https://") {
		return fmt.Errorf("refusing to fetch over anything but HTTPS: %s", base)
	}
	got := make(map[string][]byte, len(files))
	for _, name := range files {
		url := base + "/" + version + "/ucd/" + name
		data, err := download(client, url)
		if err != nil {
			return err
		}
		if err := checkVersionLine(name, data, version); err != nil {
			return err
		}
		got[name] = data
		fmt.Fprintf(stdout, "%-22s %8d bytes  sha256 %s\n", name, len(data), sumHex(data))
	}

	// Check every existing file before writing any, so a refusal leaves the
	// directory as it was.
	for _, name := range files {
		old, err := os.ReadFile(filepath.Join(dir, name))
		switch {
		case err == nil && !bytes.Equal(old, got[name]):
			return fmt.Errorf("%s already exists with different bytes (committed sha256 %s, fetched %s); not overwriting a reviewed pin",
				filepath.Join(dir, name), sumHex(old), sumHex(got[name]))
		case err != nil && !os.IsNotExist(err):
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), got[name], 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "wrote %d files to %s\n", len(files), dir)
	return nil
}

func download(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("%s: more than %d bytes", url, maxBytes)
	}
	return data, nil
}

// checkVersionLine requires the two files that carry one to open with it.
func checkVersionLine(name string, data []byte, version string) error {
	var want string
	switch name {
	case "CaseFolding.txt":
		want = "# CaseFolding-" + version + ".txt"
	case "NormalizationTest.txt":
		want = "# NormalizationTest-" + version + ".txt"
	default:
		return nil
	}
	first := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		first = data[:i]
	}
	if got := strings.TrimRight(string(first), "\r"); got != want {
		return fmt.Errorf("%s opens with %q, want %q", name, got, want)
	}
	return nil
}

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
