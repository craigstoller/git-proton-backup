package repo

import (
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
)

// refNameFixtures is the shared table both TestCheckRefNameRules and
// TestCheckRefNameParityWithGit walk: one list, so a fixture added for the
// in-process validator is automatically checked against real git too, and a
// fixture that only makes sense against real git cannot silently go
// unrepresented in the in-process rule set.
//
// accept holds names CheckRefName must accept — including "refs/heads/a{b}",
// which is the split TestAdvertisableName depends on: braces are valid to
// git (confirmed live below) but refused by advertisableName as a
// STAGEABILITY concern (checkStageableLeaf, marker.go — cli-drive@0.7.0
// still glob-expands "{", probe C13).
//
// reject holds names CheckRefName must refuse, including the round-1 catch
// (one-level names "main", "refs", "HEAD" — git's default rules require at
// least one "/") and the round-3 catch (refs/heads/.hidden — a leading-dot
// COMPONENT, not just a leading-dot whole name).
var refNameFixtures = struct {
	accept []string
	reject []string
}{
	accept: []string{
		"refs/heads/main",
		"refs/heads/feature/x",
		"refs/tags/v1/rc",
		"refs/notes/commits",
		"refs/stash",
		"refs/heads/a.b",
		"refs/heads/a{b}",
	},
	reject: []string{
		"refs/heads/.hidden",
		"refs/heads/a..b",
		"refs/heads/a.lock",
		"refs/heads/a/",
		"refs//x",
		"refs/heads/a\\b",
		"refs/heads/@{",
		"refs/heads/a b",
		"refs/heads/a:b",
		"refs/heads/a~b",
		"refs/heads/a^b",
		"refs/heads/a?b",
		"refs/heads/a*b",
		"refs/heads/a[b",
		"@",
		"refs/heads/a.",
		"refs/heads/a\x01b", // control character
		"",
		"/refs/heads/x",
		"main",
		"refs",
		"HEAD",
	},
}

// TestCheckRefNameRules: RED, table test, both directions — see
// refNameFixtures above for the full accept/reject list and the reasoning
// behind each entry.
func TestCheckRefNameRules(t *testing.T) {
	for _, name := range refNameFixtures.accept {
		if err := CheckRefName(name); err != nil {
			t.Errorf("CheckRefName(%q) = %v, want nil (accept)", name, err)
		}
	}
	for _, name := range refNameFixtures.reject {
		if err := CheckRefName(name); err == nil {
			t.Errorf("CheckRefName(%q) = nil, want a refusal (reject)", name)
		}
	}
}

// TestCheckRefNameParityWithGit is the round-3 mandate: every fixture above,
// accept AND reject, run through the real gitcmd.CheckRefFormat, and the
// verdict must match CheckRefName's. Drift between the in-process validator
// and git's own rule set is a test failure here, not a silent divergence —
// this is what keeps CheckRefName honest as git's rule set is the normative
// bar, not the prose list in its doc comment.
func TestCheckRefNameParityWithGit(t *testing.T) {
	checked := 0
	for _, name := range refNameFixtures.accept {
		checked++
		gitOK, err := gitcmd.CheckRefFormat(name)
		if err != nil {
			t.Errorf("gitcmd.CheckRefFormat(%q): %v", name, err)
			continue
		}
		if !gitOK {
			t.Errorf("real git rejects accept-fixture %q; CheckRefName and git have diverged", name)
		}
		if err := CheckRefName(name); err != nil {
			t.Errorf("CheckRefName(%q) = %v, but real git accepts it", name, err)
		}
	}
	for _, name := range refNameFixtures.reject {
		checked++
		gitOK, err := gitcmd.CheckRefFormat(name)
		if err != nil {
			t.Errorf("gitcmd.CheckRefFormat(%q): %v", name, err)
			continue
		}
		if gitOK {
			t.Errorf("real git accepts reject-fixture %q; CheckRefName and git have diverged", name)
		}
		if err := CheckRefName(name); err == nil {
			t.Errorf("CheckRefName(%q) = nil, but real git rejects it", name)
		}
	}
	t.Logf("parity-checked %d fixtures against real git", checked)
}

// TestAdvertisableName: RED. advertisableName additionally rejects brace
// components (the stageability refusal CheckRefName alone does not apply —
// see refNameFixtures' "refs/heads/a{b}" accept case above) and Windows
// device-name components, and accepts everything CheckRefName-valid whose
// components are all stageable.
func TestAdvertisableName(t *testing.T) {
	// Every CheckRefName-accept fixture except the brace one must also be
	// advertisable: its components are all ordinary, stageable names.
	for _, name := range refNameFixtures.accept {
		if name == "refs/heads/a{b}" {
			continue // covered by the brace-specific cases below
		}
		if err := advertisableName(name); err != nil {
			t.Errorf("advertisableName(%q) = %v, want nil", name, err)
		}
	}

	rejectCases := []string{
		"refs/heads/a{b}",   // brace component: git-valid, not stageable
		"refs/heads/a{b}/c", // brace component, not in leaf position
		"refs/heads/con/x",  // Windows-reserved device name mid-path
		"refs/heads/x/aux",  // Windows-reserved device name as leaf
	}
	for _, name := range rejectCases {
		if err := advertisableName(name); err == nil {
			t.Errorf("advertisableName(%q) = nil, want a refusal", name)
		}
	}

	// A CheckRefName-invalid name must also be refused here — advertisableName
	// does not loosen anything CheckRefName already refuses.
	for _, name := range refNameFixtures.reject {
		if err := advertisableName(name); err == nil {
			t.Errorf("advertisableName(%q) = nil, want a refusal (CheckRefName already refuses it)", name)
		}
	}
}
