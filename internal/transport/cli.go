package transport

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type CLI struct{ Exe string }

func NewCLI(exe string) *CLI {
	if exe == "" {
		exe = "proton-drive"
	}
	return &CLI{Exe: exe}
}

func (c *CLI) run(args ...string) (string, int, error) {
	cmd := exec.Command(c.Exe, args...)
	out, err := cmd.CombinedOutput()
	// If the executable never started (not on PATH, permission denied, etc.),
	// cmd.ProcessState is nil and ExitCode() would panic. Fail closed instead:
	// report a non-zero code and the start error.
	if cmd.ProcessState == nil {
		return string(out), -1, err
	}
	code := cmd.ProcessState.ExitCode()
	return string(out), code, err
}

// nodeWire mirrors the CLI payload. activeRevision is json.RawMessage because
// its shape differs by version and must be decoded in two attempts.
type nodeWire struct {
	Name struct {
		Value string `json:"value"`
	} `json:"name"`
	Type           string          `json:"type"`
	ActiveRevision json.RawMessage `json:"activeRevision"`
}

type revWire struct {
	State       string `json:"state"`
	ClaimedSize int64  `json:"claimedSize"`
}

func parseNodeJSON(b []byte) (Node, error) {
	var w nodeWire
	if err := json.Unmarshal(b, &w); err != nil {
		return Node{}, err
	}
	n := Node{Name: w.Name.Value, IsDir: w.Type == "folder"}
	if len(w.ActiveRevision) > 0 {
		var r revWire
		// 0.7.0: unwrapped.
		if err := json.Unmarshal(w.ActiveRevision, &r); err == nil && r.State != "" {
			n.Size = r.ClaimedSize
			return n, nil
		}
		// 0.4.6: {ok, value}.
		var wrap struct {
			OK    bool    `json:"ok"`
			Value revWire `json:"value"`
		}
		if err := json.Unmarshal(w.ActiveRevision, &wrap); err == nil && wrap.OK {
			n.Size = wrap.Value.ClaimedSize
		}
	}
	return n, nil
}

func (c *CLI) Stat(p string) (Node, bool, error) {
	out, code, _ := c.run("filesystem", "info", p, "--json")
	if code != 0 {
		return Node{}, false, nil // absence is not an error
	}
	n, err := parseNodeJSON([]byte(out))
	if err != nil {
		return Node{}, false, fmt.Errorf("unparseable info for %s: %w", p, err)
	}
	return n, true, nil
}

func (c *CLI) List(p string) ([]Node, error) {
	out, code, _ := c.run("filesystem", "list", p, "--json")
	if code != 0 {
		return nil, fmt.Errorf("list %s failed: %s", p, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) == "" {
		return []Node{}, nil // empty folder: exit 0, no output (Stage 1 C6)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("unparseable listing for %s: %w", p, err)
	}
	nodes := make([]Node, 0, len(raw))
	for _, r := range raw {
		n, err := parseNodeJSON(r)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (c *CLI) ReadTo(p, localDir string) error {
	out, code, _ := c.run("filesystem", "download", p, localDir)
	if code != 0 {
		return fmt.Errorf("download %s failed: %s", p, strings.TrimSpace(out))
	}
	return nil
}

// EnsureDir is Stat-then-create: create-folder exits 1 on an existing folder
// (Stage 1 C5). Swallowing that error generically would also hide real
// permission and path failures, so the existence check is explicit.
func (c *CLI) EnsureDir(p string) error {
	if _, ok, err := c.Stat(p); err != nil {
		return err
	} else if ok {
		return nil
	}
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return fmt.Errorf("refusing to create a root-level folder: %s", p)
	}
	parent, name := p[:i], p[i+1:]
	out, code, _ := c.run("filesystem", "create-folder", parent, name)
	if code != 0 {
		return fmt.Errorf("create-folder %s in %s failed: %s", name, parent, strings.TrimSpace(out))
	}
	return nil
}
