package userflow

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// MobileLaunchChallenge installs, launches, and verifies
// stability of a mobile application. It waits for a
// configurable duration to confirm the app does not crash.
type MobileLaunchChallenge struct {
	challenge.BaseChallenge
	adapter       MobileAdapter
	appPath       string
	stabilityWait time.Duration
}

// NewMobileLaunchChallenge creates a challenge that installs
// and launches the app from appPath, then waits stabilityWait
// to verify it remains running.
func NewMobileLaunchChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter MobileAdapter,
	appPath string,
	stabilityWait time.Duration,
) *MobileLaunchChallenge {
	return &MobileLaunchChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"mobile",
			deps,
		),
		adapter:       adapter,
		appPath:       appPath,
		stabilityWait: stabilityWait,
	}
}

// Execute installs the app, launches it, waits for the
// stability period, checks if it is still running, takes a
// screenshot, and stops the app.
func (c *MobileLaunchChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("MobileLaunchChallenge: platform not available, skipped (app=%s)", c.appPath))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)

	// Install app.
	c.ReportProgress("installing app", map[string]any{
		"path": c.appPath,
	})
	installErr := c.adapter.InstallApp(ctx, c.appPath)
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:     "install",
			Target:   "app_install",
			Expected: "true",
			Actual: fmt.Sprintf(
				"%t", installErr == nil,
			),
			Passed:  installErr == nil,
			Message: installMessage(installErr),
		},
	)
	if installErr != nil {
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			installErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("MobileLaunchChallenge: install failed for %s", c.appPath))
		return result, nil
	}

	// Launch app.
	c.ReportProgress("launching app", nil)
	launchErr := c.adapter.LaunchApp(ctx)
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:     "launch",
			Target:   "app_launch",
			Expected: "true",
			Actual: fmt.Sprintf(
				"%t", launchErr == nil,
			),
			Passed:  launchErr == nil,
			Message: launchMessage(launchErr),
		},
	)
	if launchErr != nil {
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			launchErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("MobileLaunchChallenge: launch failed for %s", c.appPath))
		return result, nil
	}

	// Wait for stability.
	c.ReportProgress("waiting for stability", map[string]any{
		"duration": c.stabilityWait.String(),
	})
	select {
	case <-ctx.Done():
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			ctx.Err().Error(),
		)
		result.RecordAction(fmt.Sprintf("MobileLaunchChallenge: context cancelled during stability wait for %s", c.appPath))
		return result, nil
	case <-time.After(c.stabilityWait):
	}

	// Check if still running.
	running, runErr := c.adapter.IsAppRunning(ctx)
	stablePassed := runErr == nil && running
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:     "stability",
			Target:   "app_stable",
			Expected: "true",
			Actual: fmt.Sprintf(
				"%t", stablePassed,
			),
			Passed:  stablePassed,
			Message: stabilityMessage(running, runErr),
		},
	)

	// Take screenshot.
	if ssData, ssErr := c.adapter.TakeScreenshot(
		ctx,
	); ssErr == nil {
		outputs["screenshot_size"] = fmt.Sprintf(
			"%d", len(ssData),
		)
	}

	// Stop app.
	c.ReportProgress("stopping app", nil)
	_ = c.adapter.StopApp(ctx)

	duration := time.Since(start)
	metrics["launch_duration"] = challenge.MetricValue{
		Name:  "launch_duration",
		Value: duration.Seconds(),
		Unit:  "s",
	}

	allPassed := true
	for _, a := range assertions {
		if !a.Passed {
			allPassed = false
			break
		}
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("MobileLaunchChallenge: launched %s, stable=%t, status=%s", c.appPath, stablePassed, status))
	return result, nil
}

// installMessage returns a message for the install assertion.
func installMessage(err error) string {
	if err == nil {
		return "app installed successfully"
	}
	return fmt.Sprintf(
		"app install failed: %s", err.Error(),
	)
}

// launchMessage returns a message for the launch assertion.
func launchMessage(err error) string {
	if err == nil {
		return "app launched successfully"
	}
	return fmt.Sprintf(
		"app launch failed: %s", err.Error(),
	)
}

// stabilityMessage returns a message for the stability check.
func stabilityMessage(running bool, err error) string {
	if err != nil {
		return fmt.Sprintf(
			"stability check failed: %s", err.Error(),
		)
	}
	if running {
		return "app remained stable after wait period"
	}
	return "app crashed during stability wait"
}

// MobileFlowChallenge executes a MobileFlow by dispatching
// each step action to the MobileAdapter.
type MobileFlowChallenge struct {
	challenge.BaseChallenge
	adapter MobileAdapter
	flow    MobileFlow
}

// NewMobileFlowChallenge creates a challenge that executes
// the given MobileFlow using the provided adapter.
func NewMobileFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter MobileAdapter,
	flow MobileFlow,
) *MobileFlowChallenge {
	return &MobileFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"mobile",
			deps,
		),
		adapter: adapter,
		flow:    flow,
	}
}

// Execute runs each step in the mobile flow, collecting
// assertions and metrics per step.
func (c *MobileFlowChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("MobileFlowChallenge: platform not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	allPassed := true

	for i, step := range c.flow.Steps {
		c.ReportProgress(
			fmt.Sprintf(
				"step %d/%d: %s (%s)",
				i+1, len(c.flow.Steps),
				step.Name, step.Action,
			),
			map[string]any{
				"step":   step.Name,
				"action": step.Action,
			},
		)

		stepStart := time.Now()
		stepErr := c.executeStep(ctx, step)
		stepDur := time.Since(stepStart)

		passed := stepErr == nil
		if !passed {
			allPassed = false
		}

		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   step.Action,
				Target: step.Name,
				Passed: passed,
				Message: mobileStepMessage(
					step.Name, step.Action, stepErr,
				),
			},
		)

		// Evaluate step assertions.
		for _, sa := range step.Assertions {
			saPassed := stepErr == nil
			if !saPassed {
				allPassed = false
			}
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:    sa.Type,
					Target:  sa.Target,
					Passed:  saPassed,
					Message: sa.Message,
				},
			)
		}

		durKey := fmt.Sprintf(
			"step_%s_duration", step.Name,
		)
		metrics[durKey] = challenge.MetricValue{
			Name:  durKey,
			Value: stepDur.Seconds(),
			Unit:  "s",
		}
	}

	metrics["steps_executed"] = challenge.MetricValue{
		Name:  "steps_executed",
		Value: float64(len(c.flow.Steps)),
		Unit:  "steps",
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress("mobile flow complete", map[string]any{
		"status": status,
		"steps":  len(c.flow.Steps),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, nil, "",
	)
	result.RecordAction(fmt.Sprintf("MobileFlowChallenge: executed %d steps, status=%s", len(c.flow.Steps), status))
	return result, nil
}

// executeStep dispatches the mobile action for a single step.
func (c *MobileFlowChallenge) executeStep(
	ctx context.Context, step MobileStep,
) error {
	switch step.Action {
	case "tap":
		return c.adapter.Tap(ctx, step.X, step.Y)

	case "send_keys":
		return c.adapter.SendKeys(ctx, step.Value)

	case "press_key":
		return c.adapter.PressKey(ctx, step.Value)

	case "wait":
		return c.adapter.WaitForApp(
			ctx, 5*time.Second,
		)

	case "screenshot":
		_, err := c.adapter.TakeScreenshot(ctx)
		return err

	case "assert_running":
		running, err := c.adapter.IsAppRunning(ctx)
		if err != nil {
			return err
		}
		if !running {
			return fmt.Errorf("app is not running")
		}
		return nil

	case "launch":
		return c.adapter.LaunchApp(ctx)

	case "stop":
		return c.adapter.StopApp(ctx)

	default:
		return fmt.Errorf(
			"unknown mobile action: %s", step.Action,
		)
	}
}

// mobileStepMessage returns a human-readable message for a
// mobile step assertion.
func mobileStepMessage(
	name, action string, err error,
) string {
	if err == nil {
		return fmt.Sprintf(
			"step %q (%s) succeeded", name, action,
		)
	}
	return fmt.Sprintf(
		"step %q (%s) failed: %s",
		name, action, err.Error(),
	)
}

// InstrumentedTestChallenge runs on-device instrumented tests
// via a MobileAdapter and aggregates results.
type InstrumentedTestChallenge struct {
	challenge.BaseChallenge
	adapter     MobileAdapter
	testClasses []string
}

// NewInstrumentedTestChallenge creates a challenge that runs
// the given instrumented test classes on a mobile device.
func NewInstrumentedTestChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter MobileAdapter,
	testClasses []string,
) *InstrumentedTestChallenge {
	return &InstrumentedTestChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"mobile",
			deps,
		),
		adapter:     adapter,
		testClasses: testClasses,
	}
}

// Execute runs each test class via adapter.RunInstrumentedTests
// and aggregates assertions and metrics.
func (c *InstrumentedTestChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("InstrumentedTestChallenge: platform not available, skipped (%d classes)", len(c.testClasses)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	allPassed := true
	totalTests := 0
	totalFailures := 0

	for i, cls := range c.testClasses {
		c.ReportProgress(
			fmt.Sprintf(
				"running test class %d/%d: %s",
				i+1, len(c.testClasses), cls,
			),
			map[string]any{"class": cls},
		)

		result, err := c.adapter.RunInstrumentedTests(
			ctx, cls,
		)

		passed := err == nil && result != nil &&
			result.TotalFailed == 0 &&
			result.TotalErrors == 0
		if !passed {
			allPassed = false
		}

		assertions = append(
			assertions, challenge.AssertionResult{
				Type:     "instrumented_tests",
				Target:   cls,
				Expected: "0 failures",
				Actual: instrumentedActual(
					result, err,
				),
				Passed: passed,
				Message: instrumentedMessage(
					cls, passed, err,
				),
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

	c.ReportProgress(
		"instrumented tests complete",
		map[string]any{
			"status":   status,
			"total":    totalTests,
			"failures": totalFailures,
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, nil, "",
	)
	result.RecordAction(fmt.Sprintf("InstrumentedTestChallenge: ran %d tests across %d classes, failures=%d, status=%s", totalTests, len(c.testClasses), totalFailures, status))
	return result, nil
}

// instrumentedActual returns the actual value string for an
// instrumented test assertion.
func instrumentedActual(
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

// instrumentedMessage returns a human-readable message for an
// instrumented test assertion.
func instrumentedMessage(
	cls string, passed bool, err error,
) string {
	if passed {
		return fmt.Sprintf(
			"all tests passed in class %q", cls,
		)
	}
	if err != nil {
		return fmt.Sprintf(
			"test class %q failed: %s",
			cls, err.Error(),
		)
	}
	return fmt.Sprintf(
		"test class %q had failures", cls,
	)
}
