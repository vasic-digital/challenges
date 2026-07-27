// Package challenges_chaos_test -- chaos tests for digital.vasic.challenges
// (§11.4.85).
//
// Fault-injection and boundary-corruption tests: nil pointers, empty configs,
// extreme timeouts, concurrent corrupt writes, and malformed assertion
// definitions. Must never panic, must always degrade gracefully.
package challenges_chaos_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	challenge "digital.vasic.challenges/pkg/challenge"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func chaosEvidenceDir() string {
	if d := os.Getenv("HELIX_STRESS_EVIDENCE_DIR"); d != "" {
		return d
	}
	return "qa-results/stress_chaos"
}

func writeChaosEvidence(t *testing.T, name string, data []byte) {
	t.Helper()
	dir := chaosEvidenceDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Logf("WARNING: mkdir %s: %v", dir, err)
		return
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	p := filepath.Join(dir, fmt.Sprintf("%s-%s.json", name, ts))
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Logf("WARNING: write evidence %s: %v", p, err)
	}
}

// catchPanic returns true if fn panics.
func catchPanic(fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	fn()
	return false
}

// ---------------------------------------------------------------------------
// TestChaosNilAndEmptyInputs
//
// Feed the challenge API with nil and empty inputs. Must not panic.
// ---------------------------------------------------------------------------

func TestChaosNilAndEmptyInputs(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chaos test skipped in short mode") // SKIP-OK: #short-mode-chaos
	}

	t.Run("nil-config", func(t *testing.T) {
		b := challenge.NewBaseChallenge("chaos-nil-cfg", "Nil Config",
			"chaos test", "chaos", nil)
		err := b.Configure(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("nil-logger-cleanup", func(t *testing.T) {
		b := challenge.NewBaseChallenge("chaos-nil-log", "Nil Logger",
			"chaos test", "chaos", nil)
		b.SetLogger(nil)
		err := b.Cleanup(context.Background())
		if err != nil {
			t.Logf("Cleanup with nil logger: %v (allowed)", err)
		}
	})

	t.Run("nil-progress-reporter", func(t *testing.T) {
		b := challenge.NewBaseChallenge("chaos-nil-prog", "Nil Progress",
			"chaos test", "chaos", nil)
		if panicked := catchPanic(func() {
			b.ReportProgress("test", nil)
		}); panicked {
			t.Fatal("ReportProgress with nil reporter panicked")
		}
	})

	t.Run("empty-result-ValidateAntiBluff", func(t *testing.T) {
		err := challenge.ValidateAntiBluff(nil)
		if err == nil {
			t.Fatal("expected error for nil result")
		}
	})

	t.Run("record-action-on-nil-result", func(t *testing.T) {
		var r *challenge.Result
		if panicked := catchPanic(func() {
			r.RecordAction("should not panic")
		}); panicked {
			t.Fatal("RecordAction on nil Result panicked")
		}
	})

	t.Run("empty-id-config", func(t *testing.T) {
		b := challenge.NewBaseChallenge("chaos-empty-id", "Empty ID",
			"chaos test", "chaos", nil)
		cfg := challenge.NewConfig("")
		cfg.ResultsDir = t.TempDir()
		cfg.LogsDir = t.TempDir()
		err := b.Configure(cfg)
		if err != nil {
			t.Fatalf("Configure with empty ID failed: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestChaosConfigEdgeCases
//
// Extreme Config values: very long paths, negative timeout, future timeouts.
// ---------------------------------------------------------------------------

func TestChaosConfigEdgeCases(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chaos test skipped in short mode") // SKIP-OK: #short-mode-chaos
	}

	t.Run("extreme-long-path", func(t *testing.T) {
		// A genuinely long BUT filesystem-valid path: several nested
		// components, each within NAME_MAX (255 bytes) and the whole
		// path within PATH_MAX (4096). Configure must create it.
		// The prior form used string(make([]byte, 500)) -- 500 NUL
		// bytes, which is a single component that is BOTH illegal (NUL
		// is forbidden in Unix path components -> EINVAL) and far over
		// NAME_MAX, so it could never succeed on any Linux filesystem
		// -- a test-bug failure, not a product defect.
		longComponent := strings.Repeat("a", 200)
		longPath := filepath.Join(
			t.TempDir(), longComponent, longComponent, longComponent,
		)
		b := challenge.NewBaseChallenge("chaos-long-path", "Long Path",
			"chaos test", "chaos", nil)
		cfg := &challenge.Config{
			ChallengeID: "chaos-long-path",
			ResultsDir:  longPath,
			LogsDir:     t.TempDir(),
			Timeout:     5 * time.Second,
		}
		if err := b.Configure(cfg); err != nil {
			t.Fatalf("extreme-long-path: Configure failed: %v", err)
		}
	})

	t.Run("invalid-path-degrades-gracefully", func(t *testing.T) {
		// Chaos contract (package doc): "must always degrade
		// gracefully" -- an unusable path must yield an error, never a
		// panic. A NUL byte is illegal in every Unix path component.
		b := challenge.NewBaseChallenge("chaos-bad-path", "Bad Path",
			"chaos test", "chaos", nil)
		cfg := &challenge.Config{
			ChallengeID: "chaos-bad-path",
			ResultsDir:  filepath.Join(t.TempDir(), "bad\x00name"),
			LogsDir:     t.TempDir(),
			Timeout:     5 * time.Second,
		}
		if err := b.Configure(cfg); err == nil {
			t.Fatal("invalid-path: expected Configure to return an " +
				"error for a NUL-containing path, got nil")
		}
	})

	t.Run("unconfigured-validate", func(t *testing.T) {
		b := challenge.NewBaseChallenge("chaos-unconfig", "Unconfigured",
			"chaos test", "chaos", nil)
		err := b.Validate(context.Background())
		if err == nil {
			t.Fatal("expected error when validating unconfigured challenge")
		}
	})
}

// ---------------------------------------------------------------------------
// TestChaosResultCorruption
//
// Construct Results with empty statuses, negative durations, nil maps, and
// concurrent writes.
// ---------------------------------------------------------------------------

func TestChaosResultCorruption(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chaos test skipped in short mode") // SKIP-OK: #short-mode-chaos
	}

	t.Run("empty-result", func(t *testing.T) {
		r := &challenge.Result{}
		if r.IsFinal() {
			t.Log("empty result IsFinal=true (allows flexibility)")
		}
	})

	t.Run("nil-maps-dont-panic", func(t *testing.T) {
		r := &challenge.Result{
			ChallengeID: "chaos-nil-maps",
			Status:      challenge.StatusPassed,
		}
		if panicked := catchPanic(func() {
			_ = r.AllPassed()
		}); panicked {
			t.Fatal("AllPassed on result with nil maps panicked")
		}
	})

	t.Run("concurrent-record-action", func(t *testing.T) {
		r := &challenge.Result{
			ChallengeID:     "chaos-concur-write",
			Status:          challenge.StatusPassed,
			RecordedActions: []string{"init"},
		}

		var wg sync.WaitGroup
		const workers = 20
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				r.RecordAction(fmt.Sprintf("chaos-action-%d", id))
			}(i)
		}
		wg.Wait()

		if len(r.RecordedActions) < workers {
			t.Logf("Concurrent RecordAction: got %d actions (expected at least %d)",
				len(r.RecordedActions), workers)
		}
	})
}

// ---------------------------------------------------------------------------
// TestChaosProgressReporterBoundaries
//
// Edge cases for ProgressReporter: rapid updates, close-then-report,
// concurrent read/write.
// ---------------------------------------------------------------------------

func TestChaosProgressReporterBoundaries(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chaos test skipped in short mode") // SKIP-OK: #short-mode-chaos
	}

	t.Run("rapid-1000-updates", func(t *testing.T) {
		p := challenge.NewProgressReporter()
		defer p.Close()

		for i := 0; i < 1000; i++ {
			p.ReportProgress(fmt.Sprintf("update %d", i),
				map[string]any{"iter": i})
		}

		last := p.LastUpdate()
		if last == nil {
			t.Fatal("LastUpdate is nil after 1000 reports")
		}
	})

	t.Run("close-is-idempotent", func(t *testing.T) {
		p := challenge.NewProgressReporter()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.Close()
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent-read-and-write", func(t *testing.T) {
		p := challenge.NewProgressReporter()
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				p.ReportProgress(fmt.Sprintf("w %d", i),
					map[string]any{"i": i})
			}
		}()

		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = p.LastUpdate()
				select {
				case <-p.Channel():
				default:
				}
			}
		}()

		wg.Wait()
		p.Close()
	})
}

// ---------------------------------------------------------------------------
// TestChaosAntiBluffCorruption
//
// ValidateAntiBluff with every class of corrupted/bluff result.
// ---------------------------------------------------------------------------

func TestChaosAntiBluffCorruption(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chaos test skipped in short mode") // SKIP-OK: #short-mode-chaos
	}

	type abCase struct {
		name      string
		result    *challenge.Result
		expectErr bool
	}
	cases := []abCase{
		{"nil-result", nil, true},
		{"empty-passed", &challenge.Result{
			ChallengeID: "ab-001",
			Status:      challenge.StatusPassed,
		}, true},
		{"failed-result", &challenge.Result{
			ChallengeID: "ab-002",
			Status:      challenge.StatusFailed,
			Error:       "real failure",
		}, false},
		{"actions-but-no-assertions", &challenge.Result{
			ChallengeID:     "ab-003",
			Status:          challenge.StatusPassed,
			RecordedActions: []string{"did something"},
		}, true},
		{"all-assertions-failed", &challenge.Result{
			ChallengeID:     "ab-004",
			Status:          challenge.StatusPassed,
			RecordedActions: []string{"action"},
			Assertions: []challenge.AssertionResult{
				{Passed: false, Message: "failed"},
			},
		}, true},
		{"valid-result", &challenge.Result{
			ChallengeID:     "ab-005",
			Status:          challenge.StatusPassed,
			RecordedActions: []string{"action"},
			Assertions: []challenge.AssertionResult{
				{Type: "not_empty", Target: "output", Passed: true, Message: "ok"},
			},
		}, false},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := challenge.ValidateAntiBluff(c.result)
			if c.expectErr && err == nil {
				t.Fatalf("%s: expected validation error, got nil", c.name)
			}
			if !c.expectErr && err != nil {
				t.Fatalf("%s: unexpected error: %v", c.name, err)
			}
		})
	}
}
