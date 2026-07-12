// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Standing race/deadlock regression guard for (*Result).RecordAction's
// concurrent-append path, fixed in commit 5bac429 ("guard
// Result.RecordAction against concurrent append + fix chaos-test path
// bug"). Constitution §11.4.169(11) (race/deadlock test-type coverage)
// and §11.4.135 (every closed defect gets a permanent regression
// guard registered in the same commit as its fix).
//
// Coverage gap this file closes: the two pre-existing concurrent
// RecordAction tests (TestChaosConcurrentResultWrite in this package
// and TestChaosResultCorruption/concurrent-record-action in
// tests/chaos) both exercise concurrent RecordAction calls and are
// caught by `go test -race`, but empirically (verified in-session,
// 2026-07-11, by reverting 5bac429's mutex and re-running the
// existing suite `-race -count=5`) they only detect the reverted race
// in 3/5 and 1/5 runs respectively at their 20-30 goroutine
// contention level -- not a deterministic (§11.4.50) regression
// signal. This file adds a higher-contention, strict-post-condition
// guard as a more reliable standing regression test, without
// duplicating or removing the existing chaos coverage.
package challenge

import (
	"fmt"
	"sync"
	"testing"
)

// TestRecordAction_ConcurrentAppend_NoLostUpdates drives
// goroutines*callsPerGoroutine concurrent calls to the REAL
// (*Result).RecordAction on one shared Result, then asserts a
// non-bluff post-condition: every single call landed (exact count,
// not "at least") and no entry was corrupted or overwritten by a
// racing append (each label is constructed to be unique; duplicates
// or an off-count are proof of slice-append corruption under a lost
// lock). It also stress-tests concurrent readers (len/iterate) of
// RecordedActions once writers have finished -- the only read pattern
// RecordAction's mutex is meant to support (pkg/runner.Runner.Run
// reads RecordedActions strictly after Execute() returns; there is no
// synchronized read accessor for RecordedActions, so overlapping an
// unsynchronized read with an in-flight write races even on the fixed
// code -- verified in-session -- which is a distinct, pre-existing API
// contract rather than the defect 5bac429 fixed).
func TestRecordAction_ConcurrentAppend_NoLostUpdates(t *testing.T) {
	t.Parallel()

	const (
		goroutines      = 100
		callsPerRoutine = 20
	)
	total := goroutines * callsPerRoutine

	r := &Result{
		ChallengeID: "race-guard-recordaction-001",
		Status:      StatusPassed,
		Assertions: []AssertionResult{
			{Type: "not_empty", Target: "output", Passed: true, Message: "output present"},
		},
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for c := 0; c < callsPerRoutine; c++ {
				r.RecordAction(fmt.Sprintf("g%03d-c%03d", gid, c))
			}
		}(g)
	}
	wg.Wait()

	// Non-bluff post-condition #1: EXACT count. With the lock, every
	// one of the `total` append calls must be recorded -- unlike the
	// pre-existing guards' "< workers" (at-least) check, equality
	// leaves no room for a partially-lost-but-still-passing run.
	if got := len(r.RecordedActions); got != total {
		t.Fatalf("lost append(s) under concurrent RecordAction: expected exactly %d recorded actions, got %d", total, got)
	}

	// Non-bluff post-condition #2: no corrupted/duplicated entries. A
	// racy append can overwrite a concurrent goroutine's slot (two
	// writers both observe the same stale len/cap and write to the
	// same index), which can leave the final length looking correct
	// by coincidence while content is corrupted (one label duplicated,
	// another silently dropped). Every label is unique by construction
	// (gid+call-index), so a duplicate or a missing label is direct
	// proof of slice-append corruption.
	seen := make(map[string]int, total)
	for _, a := range r.RecordedActions {
		seen[a]++
	}
	for label, n := range seen {
		if n != 1 {
			t.Fatalf("corrupted concurrent append: label %q recorded %d times (want 1) -- slice-append corruption under race", label, n)
		}
	}
	if len(seen) != total {
		t.Fatalf("corrupted concurrent append: %d distinct labels recorded, want %d", len(seen), total)
	}

	// Concurrent-reader stress (post-write): confirms len()/iterate
	// over RecordedActions is safe for concurrent readers once all
	// writers have completed, matching the real pkg/runner usage
	// pattern (read strictly after Execute() returns).
	var rwg sync.WaitGroup
	const readers = 20
	rwg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer rwg.Done()
			n := len(r.RecordedActions)
			for _, a := range r.RecordedActions {
				if a == "" {
					t.Error("concurrent post-write reader observed an empty action label")
				}
			}
			if n != total {
				t.Errorf("concurrent post-write reader observed %d actions, want %d", n, total)
			}
		}()
	}
	rwg.Wait()

	if err := ValidateAntiBluff(r); err != nil {
		t.Fatalf("anti-bluff validation failed after concurrent RecordAction: %v", err)
	}
}
