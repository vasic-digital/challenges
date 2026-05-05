// Fixture for CONST-035 scanner self-test. Not a real test.
// scanner-fixture: BLUFF-G-001 — t.Skip without exempt marker.
//go:build ignore

package fixtures

import "testing"

func TestBluffG001Skip(t *testing.T) {
	t.Skip()
}
