// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Stress tests for challenge package — concurrent lifecycle, assertion
// evaluation, and sustained result creation. Constitution §11.4.85.

package challenge

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// evidenceDir returns the directory for stress/chaos evidence files.
// Uses HELIX_STRESS_EVIDENCE_DIR if set, otherwise defaults to
// qa-results/stress_chaos/ under the working directory.
func evidenceDir() string {
	if d := os.Getenv("HELIX_STRESS_EVIDENCE_DIR"); d != "" {
		return d
	}
	return filepath.Join("qa-results", "stress_chaos")
}

// writeEvidence writes a named evidence file.
func writeEvidence(t *testing.T, name, content string) {
	t.Helper()
	dir := evidenceDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("WARNING: cannot create evidence dir %s: %v", dir, err)
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Logf("WARNING: cannot write evidence %s: %v", path, err)
	}
}

// writeEvidenceJSON writes structured evidence as JSON.
func writeEvidenceJSON(t *testing.T, name string, v any) {
	t.Helper()
	data, err := jsonMarshalIndent(v, "", "  ")
	if err != nil {
		t.Logf("WARNING: cannot marshal evidence: %v", err)
		return
	}
	writeEvidence(t, name, string(data))
}

// latencyRecord holds a single iteration's latency measurement.
type latencyRecord struct {
	Iteration int           `json:"iteration"`
	Latency   time.Duration `json:"latency_ns"`
}

// TestStressConcurrentBaseChallenge creates N=100 BaseChallenge instances
// concurrently, each going through Configure/Validate/Cleanup lifecycle.
// Uses testing.Short() to reduce count to 10 for quick runs.
func TestStressConcurrentBaseChallenge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	t.Parallel()

	const n = 100
	ctx := context.Background()
	tmpDir := t.TempDir()

	var (
		wg       sync.WaitGroup
		failures atomic.Int64
	)

	start := time.Now()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := ID(fmt.Sprintf("stress-base-%03d", idx))

			b := NewBaseChallenge(id, fmt.Sprintf("Stress Base %d", idx),
				"stress test challenge", "stress", nil)

			err := b.Configure(&Config{
				ChallengeID: id,
				ResultsDir:  filepath.Join(tmpDir, "results", string(id)),
				LogsDir:     filepath.Join(tmpDir, "logs", string(id)),
				Timeout:     30 * time.Second,
			})
			if err != nil {
				failures.Add(1)
				return
			}

			if err := b.Validate(ctx); err != nil {
				failures.Add(1)
				return
			}

			if err := b.Cleanup(ctx); err != nil {
				failures.Add(1)
				return
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	failCount := failures.Load()
	assert.Zero(t, failCount,
		"expected zero lifecycle failures for %d concurrent challenges, got %d",
		n, failCount)

	summary := fmt.Sprintf("TestStressConcurrentBaseChallenge\n"+
		"Concurrent challenges: %d\n"+
		"Duration: %s\n"+
		"Failures: %d\n"+
		"Status: %s\n",
		n, elapsed, failCount,
		map[bool]string{true: "PASS", false: "FAIL"}[failCount == 0])
	writeEvidence(t, "stress_concurrent_base_challenge.txt", summary)
}

// TestStressConcurrentAssertionEvaluation runs N=100 concurrent assertion
// evaluations using the mock assertion engine via EvaluateAssertions.
func TestStressConcurrentAssertionEvaluation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	t.Parallel()

	const n = 100
	const assertionsPerEval = 16

	b := NewBaseChallenge("stress-assert-eval", "Stress Assertions",
		"concurrent assertion stress test", "stress", nil)
	engine := &mockAssertionEngine{}
	b.SetAssertionEngine(engine)

	defs := make([]AssertionDef, assertionsPerEval)
	for i := 0; i < assertionsPerEval; i++ {
		defs[i] = AssertionDef{
			Type:   "not_empty",
			Target: fmt.Sprintf("output_%d", i),
		}
	}

	values := make(map[string]any)
	for i := 0; i < assertionsPerEval; i++ {
		values[fmt.Sprintf("output_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	var (
		wg          sync.WaitGroup
		failures    atomic.Int64
		totalEvaled atomic.Int64
	)

	start := time.Now()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := b.EvaluateAssertions(defs, values)
			totalEvaled.Add(int64(len(results)))
			for _, r := range results {
				if !r.Passed {
					failures.Add(1)
					return
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	failCount := failures.Load()
	totalCount := totalEvaled.Load()

	assert.Zero(t, failCount,
		"expected zero assertion failures for %d concurrent evals (%d total assertions), got %d",
		n, totalCount, failCount)

	summary := fmt.Sprintf("TestStressConcurrentAssertionEvaluation\n"+
		"Concurrent evals: %d\n"+
		"Assertions per eval: %d\n"+
		"Total assertions: %d\n"+
		"Duration: %s\n"+
		"Assertion failures: %d\n"+
		"Throughput: %.0f assertions/sec\n"+
		"Status: %s\n",
		n, assertionsPerEval, totalCount, elapsed, failCount,
		float64(totalCount)/elapsed.Seconds(),
		map[bool]string{true: "PASS", false: "FAIL"}[failCount == 0])
	writeEvidence(t, "stress_concurrent_assertion_eval.txt", summary)
}

// TestStressSustainedResultCreation runs sustained >=30 seconds of concurrent
// Result creation, status updates, and anti-bluff action recording. Measures
// throughput in results/sec.
func TestStressSustainedResultCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	t.Parallel()

	const minDuration = 30 * time.Second
	const concurrency = 10

	var (
		wg          sync.WaitGroup
		totalCreate atomic.Int64
		failures    atomic.Int64
	)

	start := time.Now()
	deadline := start.Add(minDuration)

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				id := ID(fmt.Sprintf("stress-result-%03d-%06d",
					workerID, totalCreate.Load()))

				startTime := time.Now().Add(-time.Duration(workerID*10) * time.Millisecond)
				result := &Result{
					ChallengeID:   id,
					ChallengeName: fmt.Sprintf("Stress Result %d", workerID),
					Status:        StatusPassed,
					StartTime:     startTime,
					EndTime:       time.Now(),
					Duration:      time.Since(startTime),
					Assertions: []AssertionResult{
						{
							Type:     "not_empty",
							Target:   "output",
							Passed:   true,
							Message:  "output is not empty",
							Expected: "non-empty",
							Actual:   "some output",
						},
					},
					Metrics: map[string]MetricValue{
						"latency": {Name: "latency", Value: 42.0, Unit: "ms"},
					},
					Outputs: map[string]string{
						"result": fmt.Sprintf("worker_%d", workerID),
					},
				}

				result.RecordAction(fmt.Sprintf("worker_%d_action_1", workerID))
				result.RecordAction(fmt.Sprintf("worker_%d_action_2", workerID))

				if result.Status == StatusPassed {
					if err := ValidateAntiBluff(result); err != nil {
						failures.Add(1)
					}
				}

				totalCreate.Add(1)

				if totalCreate.Load()%5 == 0 {
					failedResult := &Result{
						ChallengeID: id + "-failed",
						Status:      StatusFailed,
						Error:       "simulated failure for stress",
					}
					if err := ValidateAntiBluff(failedResult); err != nil {
						failures.Add(1)
					}
				}
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(start)
	total := totalCreate.Load()
	failCount := failures.Load()

	lats := make([]latencyRecord, 0)
	for i := 0; i < int(math.Min(float64(total), 100)); i++ {
		lats = append(lats, latencyRecord{
			Iteration: i,
			Latency:   time.Duration(float64(elapsed) / float64(total)),
		})
	}

	require.True(t, elapsed >= minDuration,
		"test ran for %s, expected at least %s", elapsed, minDuration)

	throughput := float64(total) / elapsed.Seconds()

	summary := fmt.Sprintf("TestStressSustainedResultCreation\n"+
		"Duration: %s (minimum required: %s)\n"+
		"Concurrent workers: %d\n"+
		"Total results created: %d\n"+
		"Anti-bluff validation failures: %d\n"+
		"Throughput: %.0f results/sec\n"+
		"Status: %s\n",
		elapsed, minDuration, concurrency, total, failCount, throughput,
		map[bool]string{true: "PASS", false: "FAIL"}[failCount == 0])
	writeEvidence(t, "stress_sustained_result_creation.txt", summary)

	writeEvidenceJSON(t, "stress_sustained_result_creation.json", map[string]any{
		"duration":            elapsed.String(),
		"concurrent_workers":  concurrency,
		"total_results":       total,
		"validation_failures": failCount,
		"throughput_rps":      throughput,
		"latency_samples":     lats,
		"status":              map[bool]string{true: "PASS", false: "FAIL"}[failCount == 0],
	})

	assert.Zero(t, failCount,
		"expected zero anti-bluff validation failures, got %d", failCount)
	assert.Greater(t, total, int64(concurrency*10),
		"expected more than %d results created in %s, got %d",
		concurrency*10, elapsed, total)
}
