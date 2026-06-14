package userflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CargoCLIAdapter implements BuildAdapter for Rust projects
// by shelling out to cargo commands.
type CargoCLIAdapter struct {
	projectRoot string
}

// Compile-time interface check.
var _ BuildAdapter = (*CargoCLIAdapter)(nil)

// NewCargoCLIAdapter creates a CargoCLIAdapter rooted at
// projectRoot.
func NewCargoCLIAdapter(
	projectRoot string,
) *CargoCLIAdapter {
	return &CargoCLIAdapter{projectRoot: projectRoot}
}

// Build runs `cargo build` in the project root.
func (a *CargoCLIAdapter) Build(
	ctx context.Context, target BuildTarget,
) (*BuildResult, error) {
	args := []string{"build"}
	args = append(args, target.Args...)

	start := time.Now()
	output, err := a.runCargo(ctx, args...)
	elapsed := time.Since(start)

	return &BuildResult{
		Target:   target.Name,
		Success:  err == nil,
		Duration: elapsed,
		Output:   output,
	}, err
}

// cargoTestEvent represents a single event from
// `cargo test -- --format=json`.
type cargoTestEvent struct {
	Type  string `json:"type"`
	Event string `json:"event"`
	Name  string `json:"name"`
}

// RunTests runs `cargo test` with JSON output and parses the
// results into a TestResult.
func (a *CargoCLIAdapter) RunTests(
	ctx context.Context, target TestTarget,
) (*TestResult, error) {
	args := []string{
		"test", "--",
		"--format=json",
		"-Z", "unstable-options",
	}
	if target.Filter != "" {
		args = []string{
			"test", target.Filter, "--",
			"--format=json",
			"-Z", "unstable-options",
		}
	}

	start := time.Now()
	output, runErr := a.runCargo(ctx, args...)
	elapsed := time.Since(start)

	result := parseCargoTestJSON(output, elapsed)
	return result, runErr
}

// parseCargoTestJSON parses the JSON lines output of
// `cargo test -- --format=json` and builds a TestResult.
func parseCargoTestJSON(
	output string, elapsed time.Duration,
) *TestResult {
	result := &TestResult{
		Duration: elapsed,
		Output:   output,
	}

	suite := TestSuite{Name: "cargo test"}
	scanner := bufio.NewScanner(
		strings.NewReader(output),
	)
	// A single line in the cargo test stream can exceed bufio's
	// default 64KB token limit; without a larger buffer the scanner
	// stops there and every later event — including `failed` — is
	// silently dropped, undercounting failures (a PASS-bluff).
	scanner.Buffer(
		make([]byte, 0, 1024*1024), 64*1024*1024,
	)

	for scanner.Scan() {
		line := scanner.Text()
		var ev cargoTestEvent
		if err := json.Unmarshal(
			[]byte(line), &ev,
		); err != nil {
			continue
		}

		if ev.Type != "test" || ev.Name == "" {
			continue
		}

		switch ev.Event {
		case "ok":
			suite.Tests++
			suite.TestCases = append(
				suite.TestCases, TestCase{
					Name:   ev.Name,
					Status: "passed",
				},
			)
		case "failed":
			suite.Tests++
			suite.Failures++
			suite.TestCases = append(
				suite.TestCases, TestCase{
					Name:   ev.Name,
					Status: "failed",
					Failure: &TestFailure{
						Message: "test failed",
						Type:    "assertion",
					},
				},
			)
		case "ignored":
			suite.Tests++
			suite.Skipped++
			suite.TestCases = append(
				suite.TestCases, TestCase{
					Name:   ev.Name,
					Status: "skipped",
				},
			)
		}
	}

	// A scan error means the stream was truncated and the counts
	// below are an undercount — record it so callers do not treat a
	// partial parse as a clean pass.
	if err := scanner.Err(); err != nil {
		result.TotalErrors++
	}

	if suite.Tests > 0 {
		result.Suites = append(result.Suites, suite)
		result.TotalTests = suite.Tests
		result.TotalFailed = suite.Failures
		result.TotalSkipped = suite.Skipped
	}

	return result
}

// Lint runs `cargo clippy -- -D warnings` in the project root.
func (a *CargoCLIAdapter) Lint(
	ctx context.Context, target LintTarget,
) (*LintResult, error) {
	args := []string{"clippy", "--", "-D", "warnings"}
	args = append(args, target.Args...)

	start := time.Now()
	output, err := a.runCargo(ctx, args...)
	elapsed := time.Since(start)

	warnings := 0
	errors := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "warning:") {
			warnings++
		}
		if strings.Contains(line, "error:") {
			errors++
		}
	}

	return &LintResult{
		Tool:     "cargo clippy",
		Success:  err == nil,
		Duration: elapsed,
		Warnings: warnings,
		Errors:   errors,
		Output:   output,
	}, err
}

// Available returns true if Cargo.toml exists in the project
// root and cargo is in PATH.
func (a *CargoCLIAdapter) Available(
	_ context.Context,
) bool {
	_, err := os.Stat(
		filepath.Join(a.projectRoot, "Cargo.toml"),
	)
	if err != nil {
		return false
	}
	// Check that cargo is in PATH.
	_, cargoErr := exec.LookPath("cargo")
	return cargoErr == nil
}

// runCargo executes a cargo command in the project root and
// returns combined output.
func (a *CargoCLIAdapter) runCargo(
	ctx context.Context, args ...string,
) (string, error) {
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = a.projectRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf(
			"cargo %v: %w", args, err,
		)
	}
	return string(out), nil
}
