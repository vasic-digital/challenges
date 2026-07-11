// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Anti-bluff validation for challenge.Result — Go-side parallel of
// the on-device tests/lib/anti_bluff.sh framework. Constitution §11.4
// (User mandate 2026-04-28):
//
//   "We had been in position that all tests do execute with success
//    and all Challenges as well, but in reality the most of the
//    features does not work and can't be used! ... execution of tests
//    and Challenges MUST guarantee the quality, the completion and
//    full usability by end users of the product!"
//
// This file enforces the equivalent guarantees on Challenge results:
//
//   1. POSITIVE EVIDENCE — a Challenge that records Status=Passed
//      MUST also record at least one passing assertion AND at least
//      one recorded action (proof the runtime actually executed
//      something on the device, not just inspected metadata).
//
//   2. NO METADATA-ONLY PASS — if the assertions list is empty OR
//      all assertions failed but Status=Passed, the validator returns
//      ErrBluffPass.
//
//   3. UNIQUE ACTION TRACE — each RecordAction call appends to the
//      Result.RecordedActions slice (added by this file). The runtime
//      asserts non-emptiness before allowing Status=Passed.
//
// Companion to the on-device anti-bluff framework
// (device/rockchip/rk3588/tests/lib/anti_bluff.sh) — together they
// realise the §11.4 covenant at both the test and Challenge layers.

package challenge

import (
	"errors"
	"fmt"
)

// ErrBluffPass is returned by ValidateAntiBluff when a Result claims
// Status=Passed but lacks the evidence required by §11.4.
var ErrBluffPass = errors.New("challenge result is a bluff: claims passed without captured evidence")

// RecordAction appends a single action description to the Result's
// running action trace. Called by Challenge implementations BEFORE
// each user-visible action they perform (am start, screencap,
// dumpsys query, etc). The action trace is what the validator counts
// as "the runtime actually did something" before allowing PASS.
//
// Mirrors lib/anti_bluff.sh ab_send_action() — same semantics, same
// no-action-no-pass guarantee.
func (r *Result) RecordAction(description string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.RecordedActions = append(r.RecordedActions, description)
	r.mu.Unlock()
}

// ValidateAntiBluff inspects a completed Result and returns
// ErrBluffPass if the result claims Status=Passed but doesn't carry
// the positive evidence required by Constitution §11.4. Returns nil
// for any non-Passed status (Failed/Skipped/Error are honest by
// definition — the §11.4 covenant only constrains PASS).
//
// Required for Status=Passed:
//
//   - RecordedActions is non-empty (at least one action ran).
//   - Assertions is non-empty (at least one expectation was checked).
//   - At least one assertion has Passed=true (something was
//     positively confirmed; assertion-list of all failures with
//     Status=Passed is the canonical bluff pattern this guards).
//
// Composed assertions are evaluated as a single passing assertion
// (the engine collapses them into one AssertionResult). Outputs and
// Metrics are intentionally NOT required — captured-evidence in those
// shapes is allowed via Logs/Outputs/Metrics, but RecordedActions +
// at-least-one-passing-Assertion is the minimum the validator
// enforces; richer claims are encouraged but not gated.
func ValidateAntiBluff(r *Result) error {
	if r == nil {
		return errors.New("nil result")
	}
	if r.Status != StatusPassed {
		return nil
	}
	if len(r.RecordedActions) == 0 {
		return fmt.Errorf("%w: challenge %q has Status=Passed but RecordedActions is empty (no actions recorded — the runtime did not invoke RecordAction)", ErrBluffPass, r.ChallengeID)
	}
	if len(r.Assertions) == 0 {
		return fmt.Errorf("%w: challenge %q has Status=Passed but no assertions evaluated", ErrBluffPass, r.ChallengeID)
	}
	for _, a := range r.Assertions {
		if a.Passed {
			return nil
		}
	}
	return fmt.Errorf("%w: challenge %q has Status=Passed but every assertion (%d) reports Passed=false", ErrBluffPass, r.ChallengeID, len(r.Assertions))
}
