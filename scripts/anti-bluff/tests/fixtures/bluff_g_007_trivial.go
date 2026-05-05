// Fixture for CONST-035 scanner self-test. Not a real test.
// scanner-fixture: BLUFF-G-007 — assert.True(t, true) trivial assertion.
//go:build ignore

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBluffG007Trivial(t *testing.T) {
	assert.True(t, true)
}
