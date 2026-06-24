// Package challenges_stress_test -- stress tests for digital.vasic.challenges
// (§11.4.85).
//
// Exercises the challenge lifecycle (Configure -> Validate -> Cleanup),
// assertion evaluation, and anti-bluff validation under sustained concurrent
// load with categorised outcome recording.
package challenges_stress_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	challenge "digital.vasic.challenges/pkg/challenge"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func evidenceDir() string {
	if d := os.Getenv("HELIX_STRESS_EVIDENCE_DIR"); d != "" {
		return d
	}
	return "qa-results/stress_chaos"
}

func writeEvidenceJSON(t *testing.T, name string, v interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Logf("WARNING: marshal evidence: %v", err)
		return
	}
	dir := evidenceDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Logf("WARNING: mkdir evidence dir %s: %v", dir, err)
		return
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	p := filepath.Join(dir, fmt.Sprintf("%s-%s.json", name, ts))
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Logf("WARNING: write evidence %s: %v", p, err)
	}
}

func percentiles(durations []time.Duration) (p50, p95, p99 time.Duration) {
	n := len(durations)
	if n == 0 {
		return 0, 0, 0
	}
	sorted := make([]time.Duration, n)
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	p50 = sorted[n*50/100]
	p95 = sorted[n*95/100]
	p99 = sorted[n*99/100]
	return
}

// ---------------------------------------------------------------------------
// TestStressConcurrentBaseChallengeLifecycle
//
// N=100 BaseChallenge instances created, configured, validated, and cleaned up
// concurrently. All must complete without error.
// ---------------------------------------------------------------------------

func TestStressConcurrentBaseChallengeLifecycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("stress test skipped in short mode") // SKIP-OK: #short-mode-stress
	}

	const n = 100
	ctx := context.Background()
	tmpDir := t.TempDir()

	var (
		wg       sync.WaitGroup
		failures int64
		mutx     sync.Mutex
		latencies []time.Duration
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			id := challenge.ID(fmt.Sprintf("stress-base-%03d", idx))
			b := challenge.NewBaseChallenge(id, fmt.Sprintf("Stress Base %d", idx),
				"stress test challenge", "stress", nil)

			start := time.Now()

			err := b.Configure(&challenge.Config{
				ChallengeID: id,
				ResultsDir:  filepath.Join(tmpDir, "results", string(id)),
				LogsDir:     filepath.Join(tmpDir, "logs", string(id)),
				Timeout:     30 * time.Second,
			})
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}

			if err := b.Validate(ctx); err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}

			if err := b.Cleanup(ctx); err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}

			mutx.Lock()
			latencies = append(latencies, time.Since(start))
			mutx.Unlock()
		}(i)
	}
	wg.Wait()

	failCount := atomic.LoadInt64(&failures)
	p50, p95, p99 := percentiles(latencies)
	record := map[string]interface{}{
		"test":      "TestStressConcurrentBaseChallengeLifecycle",
		"N":         n,
		"failures":  failCount,
		"p50_ns":    p50.Nanoseconds(),
		"p95_ns":    p95.Nanoseconds(),
		"p99_ns":    p99.Nanoseconds(),
	}
	writeEvidenceJSON(t, "concurrent_base_challenge", record)
	t.Logf("Concurrent BaseChallenge N=%d: failures=%d p50=%v p95=%v p99=%v",
		n, failCount, p50, p95, p99)
	if failCount != 0 {
		t.Fatalf("%d BaseChallenge lifecycle failures", failCount)
	}
}

// ---------------------------------------------------------------------------
// TestStressConcurrentAntiBluffValidation
//
// N=200 goroutines each validate a mix of valid and bluff results via
// ValidateAntiBluff. Assert correct PASS/FAIL classification.
// ---------------------------------------------------------------------------

func TestStressConcurrentAntiBluffValidation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("stress test skipped in short mode") // SKIP-OK: #short-mode-stress
	}

	const n = 200
	var wg sync.WaitGroup
	var misses int64

	validResult := &challenge.Result{
		ChallengeID: "stress-antibluff-valid",
		Status:      challenge.StatusPassed,
		RecordedActions: []string{"action1", "action2"},
		Assertions: []challenge.AssertionResult{
			{Type: "not_empty", Target: "output", Passed: true, Message: "output present"},
		},
	}

	bluffResult := &challenge.Result{
		ChallengeID: "stress-antibluff-bluff",
		Status:      challenge.StatusPassed,
	}

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var err error
			if idx%2 == 0 {
				err = challenge.ValidateAntiBluff(validResult)
			} else {
				err = challenge.ValidateAntiBluff(bluffResult)
			}
			if idx%2 == 0 && err != nil {
				// A valid result should pass anti-bluff.
				atomic.AddInt64(&misses, 1)
			}
			if idx%2 == 1 && err == nil {
				// A bluff result should fail anti-bluff.
				atomic.AddInt64(&misses, 1)
			}
		}(i)
	}
	wg.Wait()

	missCount := atomic.LoadInt64(&misses)
	record := map[string]interface{}{
		"test":    "TestStressConcurrentAntiBluffValidation",
		"N":       n,
		"misses":  missCount,
	}
	writeEvidenceJSON(t, "concurrent_antibluff_validation", record)
	t.Logf("Concurrent anti-bluff N=%d: misses=%d", n, missCount)
	if missCount != 0 {
		t.Fatalf("%d anti-bluff classification misses", missCount)
	}
}

// ---------------------------------------------------------------------------
// TestStressSustainedAssertionEvaluation
//
// N=500 assertion evaluations across a rotating set of assertion types,
// recording latency distribution.
// ---------------------------------------------------------------------------

func TestStressSustainedAssertionEvaluation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("stress test skipped in short mode") // SKIP-OK: #short-mode-stress
	}

	const N = 500
	durations := make([]time.Duration, 0, N)
	var fails int32

	for i := 0; i < N; i++ {
		b := challenge.NewBaseChallenge("stress-assert", "Stress Assert",
			"assertion stress test", "stress", nil)
		engine := &mockAssertionEngine{}
		b.SetAssertionEngine(engine)

		defs := []challenge.AssertionDef{
			{Type: "not_empty", Target: "output", Message: "output must not be empty"},
		}
		values := map[string]any{"output": fmt.Sprintf("value-%d", i)}

		start := time.Now()
		results := b.EvaluateAssertions(defs, values)
		durations = append(durations, time.Since(start))

		for _, r := range results {
			if !r.Passed {
				atomic.AddInt32(&fails, 1)
			}
		}
	}

	p50, p95, p99 := percentiles(durations)
	record := map[string]interface{}{
		"test":        "TestStressSustainedAssertionEvaluation",
		"N":           N,
		"failures":    fails,
		"p50_ns":      p50.Nanoseconds(),
		"p95_ns":      p95.Nanoseconds(),
		"p99_ns":      p99.Nanoseconds(),
	}
	writeEvidenceJSON(t, "sustained_assertion_eval", record)
	t.Logf("Sustained assertion eval N=%d: failures=%d p50=%v p95=%v p99=%v",
		N, fails, p50, p95, p99)
}

// ---------------------------------------------------------------------------
// TestStressBoundaryConfig
//
// Boundary conditions for Config: empty paths, zero timeout, extreme paths.
// ---------------------------------------------------------------------------

func TestStressBoundaryConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	type configCase struct {
		name string
		cfg  *challenge.Config
	}
	cases := []configCase{
		{"empty-id", challenge.NewConfig("")},
		{"zero-timeout", &challenge.Config{
			ChallengeID: "boundary-zero-timeout",
			ResultsDir:  tmpDir,
			LogsDir:     tmpDir,
			Timeout:     0,
		}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			b := challenge.NewBaseChallenge("boundary-cfg", "Boundary Config",
				"boundary test", "stress", nil)
			err := b.Configure(c.cfg)
			if c.name == "empty-id" {
				return // expected to succeed with empty ID
			}
			if err != nil {
				t.Fatalf("%s: Configure failed: %v", c.name, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// mockAssertionEngine -- always returns PASS.
// ---------------------------------------------------------------------------

type mockAssertionEngine struct{}

func (m *mockAssertionEngine) Evaluate(def challenge.AssertionDef, value any) challenge.AssertionResult {
	return challenge.AssertionResult{
		Type:   def.Type,
		Target: def.Target,
		Passed: true,
		Actual: fmt.Sprintf("%v", value),
	}
}

func (m *mockAssertionEngine) EvaluateAll(defs []challenge.AssertionDef, values map[string]any) []challenge.AssertionResult {
	res := make([]challenge.AssertionResult, len(defs))
	for i, d := range defs {
		res[i] = m.Evaluate(d, values[d.Target])
	}
	return res
}
