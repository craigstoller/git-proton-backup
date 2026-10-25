package repo

import (
	"os"
	"testing"
)

// TestMain removes the settle wait for the whole package. Releasing an
// in-doubt lock handle waits SettleDelay (5s in production) so a late-landing
// upload can show up before the nonce read; no test here needs a real wait,
// and the many tests that end on an in-doubt handle would otherwise each pay
// it. Tests that care about the settle itself replace the settle seam.
func TestMain(m *testing.M) {
	SettleDelay = 0
	os.Exit(m.Run())
}
