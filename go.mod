module github.com/craigstoller/git-proton-backup

// A deliberate floor, not the patch release that happened to be installed when
// this module was created (it read `go 1.26.5`, which refused to build for
// anyone on an older toolchain for no reason). The stdlib APIs used here —
// bufio, encoding/json, os/exec, regexp, strings, sort, path, path/filepath,
// crypto/rand, encoding/hex, time, and os.MkdirTemp/ReadDir/WriteFile — are
// all available from Go 1.16. 1.22 is chosen over 1.16 because it is the first
// release with per-iteration loop variables: nothing here captures a loop
// variable today, but declaring the older semantics would silently arm that
// trap for the next editor. Verified building and testing on go1.26.5.
go 1.22
