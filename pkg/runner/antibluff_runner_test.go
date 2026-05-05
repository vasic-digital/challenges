// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Anti-bluff runner-integration tests — verify that the runner
// unconditionally downgrades a bluff Status=Passed result to
// StatusFailed. Constitution §1, §6.3, §11.5.7 — v2.0.0 amendment
// (2026-05-01): the CHALLENGE_ANTIBLUFF_STRICT env gate has been
// removed; anti-bluff validation is always active.

package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
)

// TestAntiBluff_BluffPassDowngraded confirms that a bluff result
// (Status=Passed but no RecordedActions, no passing assertions) is
// downgraded to StatusFailed and the Error field surfaces ErrBluffPass.
// No env-var gating is required; this is unconditional behaviour.
func TestAntiBluff_BluffPassDowngraded(t *testing.T) {
	// Construct a stub WITHOUT RecordedActions — the canonical bluff pattern.
	s := &stubChallenge{
		id:   challenge.ID("bluff-1"),
		name: "bluff-1",
		execResult: &challenge.Result{
			Status: challenge.StatusPassed,
			Assertions: []challenge.AssertionResult{
				{Passed: true, Message: "ok"},
			},
		},
	}
	reg := setupRegistry(t, s)

	r := NewRunner(
		WithRegistry(reg),
		WithResultsDir(t.TempDir()),
	)

	result, err := r.Run(context.Background(), "bluff-1", challenge.NewConfig("bluff-1"))
	require.NoError(t, err)
	require.NotNil(t, result)

	if result.Status == challenge.StatusPassed {
		t.Fatalf("expected status downgraded from Passed; got Passed (the validator did not engage)")
	}
	if result.Status != challenge.StatusFailed {
		t.Fatalf("expected StatusFailed (the canonical bluff downgrade); got %q", result.Status)
	}
	if !strings.Contains(result.Error, "bluff") {
		t.Fatalf("expected Error to mention 'bluff'; got %q", result.Error)
	}
}

// TestAntiBluff_ValidPassPreserved confirms that a result with
// RecordedActions and passing assertions is NOT downgraded.
func TestAntiBluff_ValidPassPreserved(t *testing.T) {
	s := &stubChallenge{
		id:   challenge.ID("valid-1"),
		name: "valid-1",
		execResult: &challenge.Result{
			Status: challenge.StatusPassed,
			RecordedActions: []string{"action-1"},
			Assertions: []challenge.AssertionResult{
				{Passed: true, Message: "ok"},
			},
		},
	}
	reg := setupRegistry(t, s)

	r := NewRunner(
		WithRegistry(reg),
		WithResultsDir(t.TempDir()),
	)

	result, err := r.Run(context.Background(), "valid-1", challenge.NewConfig("valid-1"))
	require.NoError(t, err)
	require.NotNil(t, result)

	if result.Status != challenge.StatusPassed {
		t.Fatalf("expected StatusPassed for valid result; got %q (Error: %q)",
			result.Status, result.Error)
	}
}
