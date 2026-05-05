package userflow

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// BuildChallenge builds one or more targets via a BuildAdapter
// and reports success/failure per target.
type BuildChallenge struct {
	challenge.BaseChallenge
	adapter BuildAdapter
	targets []BuildTarget
}

// NewBuildChallenge creates a challenge that builds the given
// targets using the provided adapter.
func NewBuildChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter BuildAdapter,
	targets []BuildTarget,
) *BuildChallenge {
	return &BuildChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"build",
			deps,
		),
		adapter: adapter,
		targets: targets,
	}
}

// Execute iterates over build targets, calls adapter.Build for
// each, and collects per-target assertions and duration metrics.
func (c *BuildChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check infrastructure availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:    "infrastructure",
				Target:  "platform_available",
				Passed:  true,
				Message: "Platform not available - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("BuildChallenge: platform not available, skipped (%d targets)", len(c.targets)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	allPassed := true

	for i, target := range c.targets {
		c.ReportProgress(
			fmt.Sprintf(
				"building target %d/%d: %s",
				i+1, len(c.targets), target.Name,
			),
			map[string]any{"target": target.Name},
		)

		result, err := c.adapter.Build(ctx, target)

		passed := err == nil && result != nil && result.Success
		if !passed {
			allPassed = false
		}

		msg := buildAssertionMessage(target.Name, passed, err)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "build_succeeds",
			Target:   target.Name,
			Expected: "true",
			Actual:   fmt.Sprintf("%t", passed),
			Passed:   passed,
			Message:  msg,
		})

		if result != nil {
			key := fmt.Sprintf(
				"%s_build_duration", target.Name,
			)
			metrics[key] = challenge.MetricValue{
				Name:  key,
				Value: result.Duration.Seconds(),
				Unit:  "s",
			}
		}
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress("build challenge complete", map[string]any{
		"status":  status,
		"targets": len(c.targets),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, nil, "",
	)
	result.RecordAction(fmt.Sprintf("BuildChallenge: built %d targets, status=%s", len(c.targets), status))
	return result, nil
}

// buildAssertionMessage returns a human-readable message for a
// build assertion.
func buildAssertionMessage(
	target string, passed bool, err error,
) string {
	if passed {
		return fmt.Sprintf(
			"build target %q succeeded", target,
		)
	}
	if err != nil {
		return fmt.Sprintf(
			"build target %q failed: %s",
			target, err.Error(),
		)
	}
	return fmt.Sprintf(
		"build target %q failed", target,
	)
}

// UnitTestChallenge runs test suites via a BuildAdapter and
// reports pass/fail per suite with aggregated metrics.
type UnitTestChallenge struct {
	challenge.BaseChallenge
	adapter BuildAdapter
	targets []TestTarget
}

// NewUnitTestChallenge creates a challenge that runs the given
// test suites using the provided adapter.
func NewUnitTestChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter BuildAdapter,
	targets []TestTarget,
) *UnitTestChallenge {
	return &UnitTestChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"test",
			deps,
		),
		adapter: adapter,
		targets: targets,
	}
}

// Execute iterates over test targets, calls adapter.RunTests
// for each, and collects per-suite assertions and metrics.
func (c *UnitTestChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check infrastructure availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:    "infrastructure",
				Target:  "platform_available",
				Passed:  true,
				Message: "Platform not available - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("UnitTestChallenge: platform not available, skipped (%d suites)", len(c.targets)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	allPassed := true
	totalTests := 0
	totalFailures := 0

	for i, target := range c.targets {
		c.ReportProgress(
			fmt.Sprintf(
				"running tests %d/%d: %s",
				i+1, len(c.targets), target.Name,
			),
			map[string]any{"suite": target.Name},
		)

		result, err := c.adapter.RunTests(ctx, target)

		passed := err == nil && result != nil &&
			result.TotalFailed == 0 &&
			result.TotalErrors == 0
		if !passed {
			allPassed = false
		}

		msg := testAssertionMessage(
			target.Name, passed, err,
		)
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:     "all_tests_pass",
				Target:   target.Name,
				Expected: "0 failures",
				Actual:   testActualValue(result, err),
				Passed:   passed,
				Message:  msg,
			},
		)

		if result != nil {
			totalTests += result.TotalTests
			totalFailures += result.TotalFailed +
				result.TotalErrors
		}
	}

	metrics["total_tests"] = challenge.MetricValue{
		Name:  "total_tests",
		Value: float64(totalTests),
		Unit:  "tests",
	}
	metrics["total_failures"] = challenge.MetricValue{
		Name:  "total_failures",
		Value: float64(totalFailures),
		Unit:  "failures",
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress("test challenge complete", map[string]any{
		"status":   status,
		"total":    totalTests,
		"failures": totalFailures,
	})

	result := c.CreateResult(
		status, start, assertions, metrics, nil, "",
	)
	result.RecordAction(fmt.Sprintf("UnitTestChallenge: ran %d tests across %d suites, failures=%d, status=%s", totalTests, len(c.targets), totalFailures, status))
	return result, nil
}

// testAssertionMessage returns a human-readable message for a
// test suite assertion.
func testAssertionMessage(
	suite string, passed bool, err error,
) string {
	if passed {
		return fmt.Sprintf(
			"all tests passed in suite %q", suite,
		)
	}
	if err != nil {
		return fmt.Sprintf(
			"test suite %q failed: %s",
			suite, err.Error(),
		)
	}
	return fmt.Sprintf(
		"test suite %q had failures", suite,
	)
}

// testActualValue returns the actual value string for a test
// assertion.
func testActualValue(
	result *TestResult, err error,
) string {
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error())
	}
	if result == nil {
		return "no result"
	}
	return fmt.Sprintf(
		"%d failures, %d errors",
		result.TotalFailed, result.TotalErrors,
	)
}

// LintChallenge runs linters via a BuildAdapter and reports
// per-tool results.
type LintChallenge struct {
	challenge.BaseChallenge
	adapter BuildAdapter
	targets []LintTarget
}

// NewLintChallenge creates a challenge that runs the given
// lint tools using the provided adapter.
func NewLintChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter BuildAdapter,
	targets []LintTarget,
) *LintChallenge {
	return &LintChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"lint",
			deps,
		),
		adapter: adapter,
		targets: targets,
	}
}

// Execute iterates over lint targets, calls adapter.Lint for
// each, and collects per-tool assertions and metrics.
func (c *LintChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check infrastructure availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:    "infrastructure",
				Target:  "platform_available",
				Passed:  true,
				Message: "Platform not available - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("LintChallenge: platform not available, skipped (%d linters)", len(c.targets)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	allPassed := true

	for i, target := range c.targets {
		c.ReportProgress(
			fmt.Sprintf(
				"running linter %d/%d: %s",
				i+1, len(c.targets), target.Name,
			),
			map[string]any{"linter": target.Name},
		)

		result, err := c.adapter.Lint(ctx, target)

		passed := err == nil && result != nil &&
			result.Success
		if !passed {
			allPassed = false
		}

		msg := lintAssertionMessage(
			target.Name, passed, err,
		)
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:     "lint_passes",
				Target:   target.Name,
				Expected: "true",
				Actual:   fmt.Sprintf("%t", passed),
				Passed:   passed,
				Message:  msg,
			},
		)

		if result != nil {
			warnKey := fmt.Sprintf(
				"%s_warnings", target.Name,
			)
			errKey := fmt.Sprintf(
				"%s_errors", target.Name,
			)
			metrics[warnKey] = challenge.MetricValue{
				Name:  warnKey,
				Value: float64(result.Warnings),
				Unit:  "warnings",
			}
			metrics[errKey] = challenge.MetricValue{
				Name:  errKey,
				Value: float64(result.Errors),
				Unit:  "errors",
			}
		}
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress("lint challenge complete", map[string]any{
		"status":  status,
		"linters": len(c.targets),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, nil, "",
	)
	result.RecordAction(fmt.Sprintf("LintChallenge: ran %d linters, status=%s", len(c.targets), status))
	return result, nil
}

// lintAssertionMessage returns a human-readable message for a
// lint assertion.
func lintAssertionMessage(
	tool string, passed bool, err error,
) string {
	if passed {
		return fmt.Sprintf(
			"linter %q passed", tool,
		)
	}
	if err != nil {
		return fmt.Sprintf(
			"linter %q failed: %s",
			tool, err.Error(),
		)
	}
	return fmt.Sprintf(
		"linter %q reported errors", tool,
	)
}
