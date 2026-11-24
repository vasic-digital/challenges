// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

// Anti-bluff validator unit tests — verify that ValidateAntiBluff
// catches the canonical bluff patterns the on-device
// tests/lib/anti_bluff.sh framework was designed to prevent.
// Constitution §11.4 — User mandate 2026-04-28.

package challenge

import (
	"errors"
	"testing"
)

// passingAssertion is a helper to construct an AssertionResult that
// captures a single passed expectation.
func passingAssertion() AssertionResult {
	return AssertionResult{
		Type:     "equals",
		Target:   "verified-output",
		Expected: "ok",
		Actual:   "ok",
		Passed:   true,
		Message:  "verified-output equals 'ok' as expected",
	}
}

// failingAssertion is the bluff pattern's mirror — a Status=Passed
// claim with assertions that are themselves all failing.
func failingAssertion() AssertionResult {
	return AssertionResult{
		Type:    "equals",
		Target:  "expected-output",
		Passed:  false,
		Message: "expected-output mismatch",
	}
}

// TestValidate_PassWithEvidence is the happy path — a Result with
// recorded actions AND at least one passing assertion is honest.
func TestValidate_PassWithEvidence(t *testing.T) {
	r := &Result{
		ChallengeID:     "test-pass-honest",
		Status:          StatusPassed,
		RecordedActions: []string{"launch app", "tap play button", "wait 5s"},
		Assertions:      []AssertionResult{passingAssertion()},
	}
	if err := ValidateAntiBluff(r); err != nil {
		t.Fatalf("expected nil for honest pass; got %v", err)
	}
}

// TestValidate_PassWithZeroActions is THE bluff pattern: Status=Passed
// claimed but the runtime never recorded a single action — the equivalent
// of the on-device "ab_summary fails on zero ACTIONS" guard.
func TestValidate_PassWithZeroActions(t *testing.T) {
	r := &Result{
		ChallengeID: "test-pass-bluff-no-actions",
		Status:      StatusPassed,
		Assertions:  []AssertionResult{passingAssertion()},
	}
	err := ValidateAntiBluff(r)
	if err == nil {
		t.Fatal("expected ErrBluffPass for zero-actions Pass; got nil")
	}
	if !errors.Is(err, ErrBluffPass) {
		t.Fatalf("expected error to wrap ErrBluffPass; got %v", err)
	}
}

// TestValidate_PassWithEmptyAssertions catches the metadata-only
// pattern — runtime ran some actions but never asserted anything
// concrete. This is "I touched the device" without "and observed
// expected state."
func TestValidate_PassWithEmptyAssertions(t *testing.T) {
	r := &Result{
		ChallengeID:     "test-pass-bluff-no-assertions",
		Status:          StatusPassed,
		RecordedActions: []string{"some action"},
		Assertions:      []AssertionResult{},
	}
	err := ValidateAntiBluff(r)
	if err == nil {
		t.Fatal("expected ErrBluffPass for empty-assertions Pass; got nil")
	}
	if !errors.Is(err, ErrBluffPass) {
		t.Fatalf("expected error to wrap ErrBluffPass; got %v", err)
	}
}

// TestValidate_PassWithAllAssertionsFailing catches the most insidious
// bluff: Status=Passed but every Assertion has Passed=false. A
// metadata-only test where the runner forgot to honour assertion
// outcomes.
func TestValidate_PassWithAllAssertionsFailing(t *testing.T) {
	r := &Result{
		ChallengeID:     "test-pass-bluff-all-fail",
		Status:          StatusPassed,
		RecordedActions: []string{"action 1", "action 2"},
		Assertions:      []AssertionResult{failingAssertion(), failingAssertion()},
	}
	err := ValidateAntiBluff(r)
	if err == nil {
		t.Fatal("expected ErrBluffPass for all-failing-assertions Pass; got nil")
	}
	if !errors.Is(err, ErrBluffPass) {
		t.Fatalf("expected error to wrap ErrBluffPass; got %v", err)
	}
}

// TestValidate_PassWithMixedAssertions confirms that ONE passing
// assertion is sufficient — assertions can be a mix of pass and
// fail (e.g., a test that proves Surface ID is correct AND that no
// stale frame was rendered) and still honestly PASS.
func TestValidate_PassWithMixedAssertions(t *testing.T) {
	r := &Result{
		ChallengeID:     "test-pass-mixed",
		Status:          StatusPassed,
		RecordedActions: []string{"action"},
		Assertions:      []AssertionResult{failingAssertion(), passingAssertion()},
	}
	if err := ValidateAntiBluff(r); err != nil {
		t.Fatalf("expected nil for mixed-assertions Pass with at-least-one passing; got %v", err)
	}
}

// TestValidate_StatusFailedHonest — Failed/Skipped/Error are honest
// by definition. The validator MUST NOT impose evidence requirements
// on non-Pass statuses.
func TestValidate_StatusFailedHonest(t *testing.T) {
	for _, s := range []string{StatusFailed, StatusSkipped, StatusTimedOut, StatusStuck, StatusError, StatusPending} {
		r := &Result{ChallengeID: "test-non-pass", Status: s}
		if err := ValidateAntiBluff(r); err != nil {
			t.Fatalf("status %q should not require evidence; got %v", s, err)
		}
	}
}

// TestRecordAction confirms the action-recording helper appends
// to the slice and survives the round-trip needed by the runtime.
func TestRecordAction(t *testing.T) {
	r := &Result{}
	r.RecordAction("first")
	r.RecordAction("second")
	r.RecordAction("third")
	if got := len(r.RecordedActions); got != 3 {
		t.Fatalf("expected 3 actions; got %d", got)
	}
	if r.RecordedActions[0] != "first" || r.RecordedActions[2] != "third" {
		t.Fatalf("recorded actions out of order or wrong: %#v", r.RecordedActions)
	}
}

// TestRecordAction_NilReceiver is a defensive check — a runtime that
// forgets to allocate the Result MUST NOT panic on RecordAction. The
// helper silently no-ops; the validator catches the resulting empty
// slice when ValidateAntiBluff runs.
func TestRecordAction_NilReceiver(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RecordAction on nil should not panic; got %v", r)
		}
	}()
	var r *Result
	r.RecordAction("noop")
}
