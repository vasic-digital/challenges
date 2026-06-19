// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Chaos tests for challenge package — config edge cases, result corruption,
// progress reporter corner cases, anti-bluff enforcement, and concurrent
// result write safety. Constitution §11.4.85.

package challenge

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestChaosConfigValidation exercises Config edge cases and validation
// failure modes: empty fields, nil logger, missing challenge ID.
func TestChaosConfigValidation(t *testing.T) {
	t.Parallel()

	t.Run("nil config triggers configure error", func(t *testing.T) {
		t.Parallel()
		b := NewBaseChallenge("chaos-cfg-001", "Nil Config",
			"chaos test", "chaos", nil)
		err := b.Configure(nil)
		if err == nil {
			t.Fatal("expected error when configuring with nil config")
		}
	})

	t.Run("nil logger does not panic on cleanup", func(t *testing.T) {
		t.Parallel()
		b := NewBaseChallenge("chaos-cfg-002", "Nil Logger",
			"chaos test", "chaos", nil)
		b.SetLogger(nil)
		err := b.Cleanup(context.Background())
		if err != nil {
			t.Fatalf("cleanup with nil logger should succeed; got %v", err)
		}
	})

	t.Run("nil progress reporter does not panic on report", func(t *testing.T) {
		t.Parallel()
		b := NewBaseChallenge("chaos-cfg-003", "Nil Progress",
			"chaos test", "chaos", nil)
		if panicked := catchPanic(func() {
			b.ReportProgress("test", nil)
		}); panicked {
			t.Fatal("ReportProgress on nil reporter should not panic")
		}
	})

	t.Run("empty challenge ID config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		b := NewBaseChallenge("chaos-cfg-004", "Empty ID",
			"chaos test", "chaos", nil)
		cfg := NewConfig("")
		cfg.ResultsDir = tmpDir
		cfg.LogsDir = tmpDir
		// An empty ID is accepted but produces uninteresting paths.
		err := b.Configure(cfg)
		if err != nil {
			t.Fatalf("configure with empty ID should succeed; got %v", err)
		}
	})

	t.Run("timeout zero means no timeout", func(t *testing.T) {
		t.Parallel()
		b := NewBaseChallenge("chaos-cfg-005", "Zero Timeout",
			"chaos test", "chaos", nil)
		cfg := NewConfig("chaos-cfg-005")
		cfg.Timeout = 0
		if err := b.Configure(cfg); err != nil {
			t.Fatalf("configure with zero timeout should succeed; got %v", err)
		}
	})

	t.Run("unconfigured validate returns error", func(t *testing.T) {
		t.Parallel()
		b := NewBaseChallenge("chaos-cfg-006", "Unconfigured Validate",
			"chaos test", "chaos", nil)
		err := b.Validate(context.Background())
		if err == nil {
			t.Fatal("expected error when validating unconfigured challenge")
		}
	})
}

// TestChaosResultEdgeCases exercises Result construction with empty,
// nil, negative, and extreme values.
func TestChaosResultEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty result with empty status", func(t *testing.T) {
		t.Parallel()
		r := &Result{}
		if r.IsFinal() {
			t.Fatal("empty status should not be final")
		}
		if !r.AllPassed() {
			t.Fatal("empty result with no assertions should AllPassed() return true")
		}
	})

	t.Run("nil error with non-Success status is valid", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID: "chaos-res-001",
			Status:      StatusFailed,
			Error:       "",
		}
		if !r.IsFinal() {
			t.Fatal("StatusFailed should be final even with empty Error")
		}
	})

	t.Run("negative duration is accepted by struct", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID: "chaos-res-002",
			Status:      StatusPassed,
			Duration:    -1 * time.Second,
		}
		// The struct does not validate Duration — this test documents
		// that callers are responsible for sanity.
		if r.Duration >= 0 {
			t.Fatal("expected negative duration to be stored as-is")
		}
	})

	t.Run("extreme start and end times", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID: "chaos-res-003",
			Status:      StatusPassed,
			StartTime:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			Duration:    time.Hour,
		}
		if r.StartTime.IsZero() {
			t.Fatal("StartTime should be non-zero for epoch date")
		}
		if r.EndTime.IsZero() {
			t.Fatal("EndTime should be non-zero for far future date")
		}
	})

	t.Run("empty outputs and metrics maps", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID: "chaos-res-004",
			Status:      StatusPassed,
			Outputs:     map[string]string{},
			Metrics:     map[string]MetricValue{},
		}
		if len(r.Outputs) != 0 {
			t.Fatal("Outputs should be empty")
		}
		if len(r.Metrics) != 0 {
			t.Fatal("Metrics should be empty")
		}
	})
}

// TestChaosProgressReporter exercises the ProgressReporter with nil
// channel scenarios, closed channels, rapid updates, and oversized data.
func TestChaosProgressReporter(t *testing.T) {
	t.Parallel()

	t.Run("new reporter has working channel", func(t *testing.T) {
		t.Parallel()
		p := NewProgressReporter()
		if p.Channel() == nil {
			t.Fatal("new reporter should have non-nil channel")
		}
		p.Close()
	})

	t.Run("report after close does not panic", func(t *testing.T) {
		t.Parallel()
		p := NewProgressReporter()
		p.Close()
		if panicked := catchPanic(func() {
			p.ReportProgress("after close", map[string]any{"k": "v"})
		}); panicked {
			t.Fatal("ReportProgress after close should not panic")
		}
	})

	t.Run("rapid progress updates (1000) do not block", func(t *testing.T) {
		t.Parallel()
		p := NewProgressReporter()
		defer p.Close()

		for i := 0; i < 1000; i++ {
			p.ReportProgress(fmt.Sprintf("rapid update %d", i),
				map[string]any{
					"iteration": i,
					"data":      "abcdefghij" + fmt.Sprintf("%d", i),
				})
		}

		// LastUpdate should reflect the final update regardless
		// of buffering.
		last := p.LastUpdate()
		if last == nil {
			t.Fatal("expected LastUpdate to be non-nil after 1000 rapid reports")
		}
		if last.Message != "rapid update 999" {
			t.Fatalf("expected final message 'rapid update 999', got %q", last.Message)
		}
	})

	t.Run("close is idempotent across goroutines", func(t *testing.T) {
		t.Parallel()
		p := NewProgressReporter()

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

	t.Run("concurrent read and write", func(t *testing.T) {
		t.Parallel()
		p := NewProgressReporter()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				p.ReportProgress(fmt.Sprintf("write %d", i),
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

// TestChaosAntibluff verifies that the anti-bluff validation functions
// correctly reject empty/mock evidence patterns.
func TestChaosAntibluff(t *testing.T) {
	t.Parallel()

	t.Run("nil result returns error", func(t *testing.T) {
		t.Parallel()
		err := ValidateAntiBluff(nil)
		if err == nil {
			t.Fatal("expected error for nil result")
		}
	})

	t.Run("empty result with StatusPassed is bluff", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID: "chaos-ab-001",
			Status:      StatusPassed,
		}
		err := ValidateAntiBluff(r)
		if err == nil {
			t.Fatal("expected ErrBluffPass for empty passed result")
		}
	})

	t.Run("result with StatusFailed passes anti-bluff", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID: "chaos-ab-002",
			Status:      StatusFailed,
			Error:       "real failure",
		}
		err := ValidateAntiBluff(r)
		if err != nil {
			t.Fatalf("expected nil for failed status; got %v", err)
		}
	})

	t.Run("result with only actions but no assertions is bluff", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID:     "chaos-ab-003",
			Status:          StatusPassed,
			RecordedActions: []string{"did something"},
		}
		err := ValidateAntiBluff(r)
		if err == nil {
			t.Fatal("expected ErrBluffPass for passed result with no assertions")
		}
	})

	t.Run("result with all-failing assertions is bluff", func(t *testing.T) {
		t.Parallel()
		r := &Result{
			ChallengeID:     "chaos-ab-004",
			Status:          StatusPassed,
			RecordedActions: []string{"action"},
			Assertions: []AssertionResult{
				{Passed: false, Message: "failed"},
				{Passed: false, Message: "also failed"},
			},
		}
		err := ValidateAntiBluff(r)
		if err == nil {
			t.Fatal("expected ErrBluffPass for passed result with all-failing assertions")
		}
	})

	t.Run("record action on nil result does not panic", func(t *testing.T) {
		t.Parallel()
		if panicked := catchPanic(func() {
			var r *Result
			r.RecordAction("should-not-panic")
		}); panicked {
			t.Fatal("RecordAction on nil Result should not panic")
		}
	})
}

// TestChaosConcurrentResultWrite exercises N=30 goroutines concurrently
// writing to the same Result fields and verifies no panic.
func TestChaosConcurrentResultWrite(t *testing.T) {
	t.Parallel()

	r := &Result{
		ChallengeID: "chaos-concur-001",
		Status:      StatusPassed,
		Assertions: []AssertionResult{
			{Type: "not_empty", Target: "output", Passed: true, Message: "output present"},
		},
		Metrics: make(map[string]MetricValue),
		Outputs: make(map[string]string),
	}

	var wg sync.WaitGroup
	const workers = 30

	var actionCounter atomic.Int64

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.RecordAction(fmt.Sprintf("worker %d action", id))
			actionCounter.Add(1)
		}(i)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.AllPassed()
		}()
	}

	wg.Wait()

	totalActions := len(r.RecordedActions)
	if totalActions < workers {
		t.Fatalf("expected at least %d recorded actions, got %d",
			workers, totalActions)
	}

	// Validate the result after concurrent writes.
	err := ValidateAntiBluff(r)
	if err != nil && totalActions > 0 {
		t.Fatalf("anti-bluff validation failed after concurrent writes: %v", err)
	}
}

// catchPanic returns true if fn panics, false otherwise.
func catchPanic(fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	fn()
	return false
}
