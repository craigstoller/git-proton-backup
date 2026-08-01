package protocol

import "testing"

func TestParsePushBatch(t *testing.T) {
	got, err := ParsePushBatch([]string{
		"push refs/heads/main:refs/heads/main",
		"push +refs/heads/dev:refs/heads/dev",
		"push :refs/heads/gone",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 updates, got %d", len(got))
	}
	if got[0].Force {
		t.Error("plain push must not be forced")
	}
	if !got[1].Force || got[1].Src != "refs/heads/dev" {
		t.Errorf("+ prefix must set Force and be stripped, got %+v", got[1])
	}
	if got[2].Src != "" || got[2].Dst != "refs/heads/gone" {
		t.Errorf("empty src means delete, got %+v", got[2])
	}
}

func TestOptionsPoison(t *testing.T) {
	var o Options
	o.Observe("option cas refs/heads/main:abc123")
	if o.Poisoned == "" {
		t.Error("cas must poison the session: git ignores our rejection and pushes anyway")
	}
	var o2 Options
	o2.Observe("option verbosity 2")
	if o2.Poisoned != "" {
		t.Error("benign options must not poison")
	}
}
