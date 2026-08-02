package transport

import "fmt"

// Outcome is what a mutation is known to have done. A NON-NIL ERROR ALWAYS
// DOMINATES THE OUTCOME: every method here returns (Outcome, error), and the
// Outcome is meaningful only when err == nil. Implementations return Ambiguous
// alongside an error as the safe accompanying value, never as a claim the
// caller may act on, so a caller that reads the Outcome without checking the
// error first is reading a value that was never asserted.
type Outcome int

const (
	Committed Outcome = iota // definitely applied
	Refused                  // name existed; nothing changed
	Ambiguous                // unknown; MUST be reconciled by reading remote state
)

// String names the outcome. An UNKNOWN value must not render as any of the
// three real ones — least of all "ambiguous", which used to be the default
// arm. Every caller that prints an Outcome does so precisely because it did
// not recognise the value: repo.publishPack and repo.publishIdx say "upload
// returned an unrecognised outcome %s", and with the old default that read
// "an unrecognised outcome ambiguous", which is self-contradictory and hides
// the very thing the message exists to report. lock.Release's "Trash reported
// %s" and pushOne's "delete failed: outcome %s" have the same exposure.
// Printing the numeric value is the only answer that is never a lie.
func (o Outcome) String() string {
	switch o {
	case Committed:
		return "committed"
	case Refused:
		return "refused"
	case Ambiguous:
		return "ambiguous"
	default:
		return fmt.Sprintf("outcome(%d)", int(o))
	}
}

type Node struct {
	Name  string
	IsDir bool
	Size  int64
}

type Transport interface {
	// EnsureDir is Stat-then-create: create-folder FAILS on an existing folder
	// (Stage 1 C5), so a bare create would error on every run after the first.
	EnsureDir(path string) error
	List(path string) ([]Node, error)
	Stat(path string) (Node, bool, error) // absence is (_, false, nil), never an error
	// ReadTo downloads the node at path into the existing local directory
	// localPath, landing as a file named after path's own remote basename.
	// localPath is a directory, never a destination file path.
	//
	// A missing or non-directory localPath must surface as the error it
	// naturally is — implementations do not create it. This is an ENFORCED
	// contract, not an incidental property of `filesystem download`: the real
	// CLI binary does NOT behave this way on its own (confirmed live, Stage
	// 3a gate task 7 — given a missing destination it silently creates the
	// directory and succeeds), so *CLI's ReadTo stats localPath itself and
	// refuses before ever invoking the CLI. Every caller in this codebase
	// creates its temp dir with os.MkdirTemp first, so a missing destination
	// always indicates a caller bug, and an implementation that auto-created
	// the directory would hide that bug rather than surface it.
	ReadTo(path, localPath string) error
	CreateExclusive(path, localPath string) (Outcome, error)
	UpdateRevision(path, localPath string) (Outcome, error)
	// Trash on a MISSING target fails with exit 1 (Stage 1 C4), so implementations
	// must Stat first and report Committed for an already-absent node.
	Trash(path string) (Outcome, error)
}
