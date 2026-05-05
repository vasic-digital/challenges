// Fixture for CONST-035 scanner self-test. Not a real test.
// scanner-fixture: clean — no bluff.
//go:build ignore

package fixtures

import "testing"

func TestClean(t *testing.T) {
	got := 1 + 1
	want := 2
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
