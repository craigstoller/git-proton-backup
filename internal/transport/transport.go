package transport

type Outcome int

const (
	Committed Outcome = iota // definitely applied
	Refused                  // name existed; nothing changed
	Ambiguous                // unknown; MUST be reconciled by reading remote state
)

func (o Outcome) String() string {
	switch o {
	case Committed:
		return "committed"
	case Refused:
		return "refused"
	default:
		return "ambiguous"
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
	ReadTo(path, localPath string) error
	CreateExclusive(path, localPath string) (Outcome, error)
	UpdateRevision(path, localPath string) (Outcome, error)
	// Trash on a MISSING target fails with exit 1 (Stage 1 C4), so implementations
	// must Stat first and report Committed for an already-absent node.
	Trash(path string) (Outcome, error)
}
