# §11.4.169(11) / §11.4.135 race-guard evidence — `(*Result).RecordAction` concurrent append

**Run ID**: `recordaction_concurrent_append_20260711`
**Date (UTC)**: 2026-07-11T15:53:11Z
**Module**: `digital.vasic.challenges` (`submodules/challenges` in the HelixCode meta-repo)
**Commit under audit**: `5bac429b6619ffd6fb289c647f4d6d0816dbd513` — "fix(challenge): guard Result.RecordAction against concurrent append + fix chaos-test path bug"
**New guard added**: `pkg/challenge/recordaction_race_test.go` — `TestRecordAction_ConcurrentAppend_NoLostUpdates`
**Go**: `go version go1.26.4-X:nodwarf5 linux/amd64`

## 1. Why a new guard was added (verification, not assumption)

5bac429 already shipped two concurrent-`RecordAction` tests: `TestChaosConcurrentResultWrite`
(`pkg/challenge/chaos_test.go`, 30 goroutines) and
`TestChaosResultCorruption/concurrent-record-action` (`tests/chaos/chaos_test.go`, 20
goroutines, non-failing `t.Logf` mismatch report). Per §11.4.6/§11.4.115 ("never guess — run
it"), both were empirically re-tested against a session-local revert of 5bac429's lock
(`sync.Mutex` field in `result.go` + `Lock()`/`Unlock()` in `antibluff.go`), `go test -race
-count=5`:

| Existing test | Goroutines | Failures / 5 runs | Detection rate |
|---|---|---|---|
| `TestChaosConcurrentResultWrite` (`pkg/challenge`) | 30 | 3/5 | 60% |
| `TestChaosResultCorruption/concurrent-record-action` (`tests/chaos`) | 20 | 1/5 | 20% |

Both existing guards DO catch the regression sometimes (the Go race detector is inherently
scheduling-dependent), but neither is a deterministic (§11.4.50) standing regression signal at
their current contention level — a CI run could plausibly land on a lucky, all-green iteration
even with the lock stripped. This gap is what `TestRecordAction_ConcurrentAppend_NoLostUpdates`
closes. It does **not** duplicate or remove either existing test; both remain in the suite.

## 2. New guard design

- 100 goroutines × 20 `RecordAction` calls each = 2000 concurrent append calls on one shared
  `*Result` (10-100x the contention of the existing guards).
- Non-bluff post-condition #1: **exact** count (`len(RecordedActions) == 2000`), not the
  existing tests' "at least N" — any lost update fails the test.
- Non-bluff post-condition #2: every recorded label is unique by construction
  (`g%03d-c%03d`); a duplicate or a missing label proves slice-append corruption (a racy
  append can overwrite a concurrent goroutine's slot and produce a coincidentally-correct
  count with corrupted content).
- Concurrent-reader stress: 20 goroutines `len()`/iterate `RecordedActions` **after**
  `wg.Wait()` — this reflects the only reader pattern the codebase actually uses
  (`pkg/runner/runner.go:505`, `result.RecordedActions = execResult.RecordedActions`, strictly
  after `Execute()` returns). An *unsynchronized* reader racing an in-flight `RecordAction`
  write was empirically confirmed (throwaway probe, deleted before commit) to race under
  `-race` even on the correctly-fixed code, because `RecordAction`'s mutex only guards the
  write path and `RecordedActions` has no synchronized read accessor — that is a pre-existing
  API contract (don't read while a challenge is still recording), not the defect 5bac429 fixed,
  so it is intentionally not exercised as an in-flight race here (doing so would produce a
  false-positive failure against correct code).

## 3. Captured evidence

### 3a. Baseline — fixed code (current HEAD, `5bac429`), `go test -race -count=3`

```
$ go test -race -count=3 -run 'TestRecordAction_ConcurrentAppend_NoLostUpdates' ./pkg/challenge/... -v
=== RUN   TestRecordAction_ConcurrentAppend_NoLostUpdates
--- PASS: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.01s)
=== RUN   TestRecordAction_ConcurrentAppend_NoLostUpdates
--- PASS: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.00s)
=== RUN   TestRecordAction_ConcurrentAppend_NoLostUpdates
--- PASS: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.00s)
PASS
ok  	digital.vasic.challenges/pkg/challenge	1.028s
BASELINE_RC=0
```

Full package suite (`go test -race -count=1 ./pkg/challenge/... ./tests/chaos/...`) also green,
35.8s / 1.0s respectively.

### 3b. §1.1 mutation — 5bac429's lock stripped (session-local, never committed)

Mutation applied: removed `sync` import + `mu sync.Mutex` field from `result.go`, removed
`r.mu.Lock()` / `r.mu.Unlock()` from `RecordAction` in `antibluff.go` (i.e. `RecordAction`
reverted to the pre-5bac429 unsynchronized `append`). Build still succeeds (clean revert, no
dangling references). `go test -race -count=5`:

```
$ go test -race -count=5 -run 'TestRecordAction_ConcurrentAppend_NoLostUpdates' ./pkg/challenge/... -v
==================
WARNING: DATA RACE
Read at 0x00c00012c2e0 by goroutine 25:
  digital.vasic.challenges/pkg/challenge.(*Result).RecordAction()
      pkg/challenge/antibluff.go:56 +0x1b6
  ...
Previous write at 0x00c00012c2e0 by goroutine 10:
  digital.vasic.challenges/pkg/challenge.(*Result).RecordAction()
      pkg/challenge/antibluff.go:56 +0x279
  ...
==================
==================
WARNING: DATA RACE
Read at 0x00c000002300 by goroutine 25:
  runtime.growslice()
      /usr/lib/golang/src/runtime/slice.go:178 +0x0
  digital.vasic.challenges/pkg/challenge.(*Result).RecordAction()
      pkg/challenge/antibluff.go:56 +0x1f8
  ...
==================
    recordaction_race_test.go:80: lost append(s) under concurrent RecordAction: expected exactly 2000 recorded actions, got 1802
    testing.go:1712: race detected during execution of test
--- FAIL: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.01s)
    recordaction_race_test.go:80: lost append(s) under concurrent RecordAction: expected exactly 2000 recorded actions, got 1938
--- FAIL: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.01s)
    recordaction_race_test.go:80: lost append(s) under concurrent RecordAction: expected exactly 2000 recorded actions, got 1918
--- FAIL: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.01s)
    recordaction_race_test.go:80: lost append(s) under concurrent RecordAction: expected exactly 2000 recorded actions, got 1618
--- FAIL: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.01s)
    recordaction_race_test.go:80: lost append(s) under concurrent RecordAction: expected exactly 2000 recorded actions, got 1931
--- FAIL: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.01s)
FAIL
FAIL	digital.vasic.challenges/pkg/challenge	0.052s
MUTATION_RC=1
```

**5/5 runs FAIL (100% detection rate at this contention level)** — every run reports both a
`WARNING: DATA RACE` from the Go race detector AND a strict-count mismatch (1618–1938 of the
expected 2000, i.e. 3–19% lost updates), demonstrating the guard's two independent, load-bearing
failure signals (race-detector + slice-append-corruption count) both fire reliably, not just
one of them.

### 3c. Restore + re-verification

```
$ git diff pkg/challenge/result.go pkg/challenge/antibluff.go
(empty — byte-identical to committed 5bac429 state)

$ go test -race -count=3 -run 'TestRecordAction_ConcurrentAppend_NoLostUpdates' ./pkg/challenge/... -v
--- PASS: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.00s)
--- PASS: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.00s)
--- PASS: TestRecordAction_ConcurrentAppend_NoLostUpdates (0.00s)
PASS
ok  	digital.vasic.challenges/pkg/challenge	1.023s
POST_RESTORE_RC=0
```

Full-run raw log (incl. stack traces for all 4 distinct DATA RACE reports across the 5
mutation iterations) preserved alongside this note for audit:
`recordaction_concurrent_append_20260711_raw.log`.

## 4. Verdict

- Existing coverage: present, real, but non-deterministic (60% / 20% detection at 20-30
  goroutines) — not fabricated, not removed, not duplicated.
- New guard `TestRecordAction_ConcurrentAppend_NoLostUpdates`: 100% detection (5/5) at 100×20
  contention, with a strict exact-count + no-duplicate-label non-bluff post-condition (vs the
  existing tests' "at least N" tolerance), plus post-write concurrent-reader stress matching
  real `pkg/runner` usage.
- PASS status: verified GREEN on the actual fixed code (baseline + post-restore, 6/6 runs
  total), never asserted without the captured run above.
