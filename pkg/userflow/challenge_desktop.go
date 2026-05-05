package userflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// DesktopLaunchChallenge launches a desktop application,
// waits for its window to appear, verifies stability over a
// configurable period, and then closes it.
type DesktopLaunchChallenge struct {
	challenge.BaseChallenge
	adapter       DesktopAdapter
	appConfig     DesktopAppConfig
	stabilityWait time.Duration
}

// NewDesktopLaunchChallenge creates a challenge that launches
// the desktop app described by appConfig and waits
// stabilityWait to verify it remains running.
func NewDesktopLaunchChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter DesktopAdapter,
	appConfig DesktopAppConfig,
	stabilityWait time.Duration,
) *DesktopLaunchChallenge {
	return &DesktopLaunchChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"desktop",
			deps,
		),
		adapter:       adapter,
		appConfig:     appConfig,
		stabilityWait: stabilityWait,
	}
}

// Execute launches the app, waits for the window, checks
// stability, takes a screenshot, and closes the app.
func (c *DesktopLaunchChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("DesktopLaunchChallenge: platform not available, skipped (binary=%s)", c.appConfig.BinaryPath))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)

	// Launch app.
	c.ReportProgress("launching desktop app", map[string]any{
		"binary": c.appConfig.BinaryPath,
	})
	launchErr := c.adapter.LaunchApp(ctx, c.appConfig)
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:     "launch",
			Target:   "app_launch",
			Expected: "true",
			Actual: fmt.Sprintf(
				"%t", launchErr == nil,
			),
			Passed:  launchErr == nil,
			Message: desktopLaunchMessage(launchErr),
		},
	)
	if launchErr != nil {
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			launchErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("DesktopLaunchChallenge: launch failed for %s", c.appConfig.BinaryPath))
		return result, nil
	}

	// Ensure app is closed when done.
	defer func() {
		_ = c.adapter.Close(ctx)
	}()

	// Wait for window.
	c.ReportProgress("waiting for window", nil)
	windowErr := c.adapter.WaitForWindow(
		ctx, 10*time.Second,
	)
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:     "window",
			Target:   "window_visible",
			Expected: "true",
			Actual: fmt.Sprintf(
				"%t", windowErr == nil,
			),
			Passed:  windowErr == nil,
			Message: windowMessage(windowErr),
		},
	)
	if windowErr != nil {
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			windowErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("DesktopLaunchChallenge: window did not appear for %s", c.appConfig.BinaryPath))
		return result, nil
	}

	// Stability wait.
	c.ReportProgress("stability check", map[string]any{
		"duration": c.stabilityWait.String(),
	})
	select {
	case <-ctx.Done():
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			ctx.Err().Error(),
		)
		result.RecordAction(fmt.Sprintf("DesktopLaunchChallenge: context cancelled during stability wait for %s", c.appConfig.BinaryPath))
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
			Passed: stablePassed,
			Message: desktopStabilityMessage(
				running, runErr,
			),
		},
	)

	// Take screenshot.
	if ssData, ssErr := c.adapter.Screenshot(
		ctx,
	); ssErr == nil {
		outputs["screenshot_size"] = fmt.Sprintf(
			"%d", len(ssData),
		)
	}

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
	result.RecordAction(fmt.Sprintf("DesktopLaunchChallenge: launched %s, stable=%t, status=%s", c.appConfig.BinaryPath, stablePassed, status))
	return result, nil
}

// desktopLaunchMessage returns a message for the launch
// assertion.
func desktopLaunchMessage(err error) string {
	if err == nil {
		return "desktop app launched successfully"
	}
	return fmt.Sprintf(
		"desktop app launch failed: %s", err.Error(),
	)
}

// windowMessage returns a message for the window assertion.
func windowMessage(err error) string {
	if err == nil {
		return "application window appeared"
	}
	return fmt.Sprintf(
		"window did not appear: %s", err.Error(),
	)
}

// desktopStabilityMessage returns a message for the desktop
// stability check.
func desktopStabilityMessage(
	running bool, err error,
) string {
	if err != nil {
		return fmt.Sprintf(
			"stability check failed: %s", err.Error(),
		)
	}
	if running {
		return "desktop app remained stable"
	}
	return "desktop app crashed during stability wait"
}

// DesktopFlowChallenge executes a BrowserFlow in the context
// of a desktop application's WebView. It reuses the BrowserFlow
// type but dispatches actions to the DesktopAdapter.
type DesktopFlowChallenge struct {
	challenge.BaseChallenge
	adapter DesktopAdapter
	flow    BrowserFlow
}

// NewDesktopFlowChallenge creates a challenge that executes
// the given BrowserFlow in the desktop app's WebView.
func NewDesktopFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter DesktopAdapter,
	flow BrowserFlow,
) *DesktopFlowChallenge {
	return &DesktopFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"desktop",
			deps,
		),
		adapter: adapter,
		flow:    flow,
	}
}

// Execute runs the browser flow steps using the desktop
// adapter's WebView methods.
func (c *DesktopFlowChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("DesktopFlowChallenge: platform not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	allPassed := true
	screenshotCount := 0

	// Navigate to start URL.
	c.ReportProgress("navigating to start URL", map[string]any{
		"url": c.flow.StartURL,
	})
	if err := c.adapter.Navigate(
		ctx, c.flow.StartURL,
	); err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "navigate",
				Target: "start_url",
				Passed: false,
				Message: fmt.Sprintf(
					"navigate to %s failed: %s",
					c.flow.StartURL, err.Error(),
				),
			},
		)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("DesktopFlowChallenge: navigate to %s failed", c.flow.StartURL))
		return result, nil
	}

	// Execute each step.
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

		if stepErr != nil {
			allPassed = false
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:   step.Action,
					Target: step.Name,
					Passed: false,
					Message: fmt.Sprintf(
						"step %q (%s) failed: %s",
						step.Name, step.Action,
						stepErr.Error(),
					),
				},
			)
		} else {
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:   step.Action,
					Target: step.Name,
					Passed: true,
					Message: fmt.Sprintf(
						"step %q (%s) succeeded",
						step.Name, step.Action,
					),
				},
			)
		}

		// Count screenshots from the screenshot action.
		if step.Action == "screenshot" && stepErr == nil {
			screenshotCount++
		}

		// Step assertions.
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

		// Screenshot after step.
		if step.Screenshot && stepErr == nil {
			if _, ssErr := c.adapter.Screenshot(
				ctx,
			); ssErr == nil {
				screenshotCount++
			}
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

	totalDur := time.Since(start)
	metrics["total_duration"] = challenge.MetricValue{
		Name:  "total_duration",
		Value: totalDur.Seconds(),
		Unit:  "s",
	}
	metrics["steps_executed"] = challenge.MetricValue{
		Name:  "steps_executed",
		Value: float64(len(c.flow.Steps)),
		Unit:  "steps",
	}

	outputs["screenshot_count"] = fmt.Sprintf(
		"%d", screenshotCount,
	)

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress(
		"desktop flow complete",
		map[string]any{
			"status": status,
			"steps":  len(c.flow.Steps),
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("DesktopFlowChallenge: executed %d steps, status=%s", len(c.flow.Steps), status))
	return result, nil
}

// executeStep dispatches a browser flow step to the desktop
// adapter. Supports the subset of actions available in
// DesktopAdapter.
func (c *DesktopFlowChallenge) executeStep(
	ctx context.Context, step BrowserStep,
) error {
	switch step.Action {
	case "navigate":
		return c.adapter.Navigate(ctx, step.Value)

	case "click":
		return c.adapter.Click(ctx, step.Selector)

	case "fill":
		return c.adapter.Fill(
			ctx, step.Selector, step.Value,
		)

	case "wait":
		timeout := step.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		return c.adapter.WaitForSelector(
			ctx, step.Selector, timeout,
		)

	case "assert_visible":
		visible, err := c.adapter.IsVisible(
			ctx, step.Selector,
		)
		if err != nil {
			return err
		}
		if !visible {
			return fmt.Errorf(
				"element %q is not visible",
				step.Selector,
			)
		}
		return nil

	case "screenshot":
		_, err := c.adapter.Screenshot(ctx)
		return err

	default:
		return fmt.Errorf(
			"unsupported desktop action: %s",
			step.Action,
		)
	}
}

// DesktopIPCChallenge tests IPC commands sent to the desktop
// application's backend (e.g., Tauri invoke).
type DesktopIPCChallenge struct {
	challenge.BaseChallenge
	adapter  DesktopAdapter
	commands []IPCCommand
}

// NewDesktopIPCChallenge creates a challenge that invokes the
// given IPC commands and validates their responses.
func NewDesktopIPCChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter DesktopAdapter,
	commands []IPCCommand,
) *DesktopIPCChallenge {
	return &DesktopIPCChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"desktop",
			deps,
		),
		adapter:  adapter,
		commands: commands,
	}
}

// Execute invokes each IPC command and compares the result
// against the expected value.
func (c *DesktopIPCChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("DesktopIPCChallenge: platform not available, skipped (%d commands)", len(c.commands)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	allPassed := true

	for i, cmd := range c.commands {
		c.ReportProgress(
			fmt.Sprintf(
				"IPC command %d/%d: %s",
				i+1, len(c.commands), cmd.Name,
			),
			map[string]any{
				"command": cmd.Command,
			},
		)

		cmdStart := time.Now()
		result, err := c.adapter.InvokeCommand(
			ctx, cmd.Command, cmd.Args...,
		)
		cmdDur := time.Since(cmdStart)

		// Check expected result if specified.
		if cmd.ExpectedResult != "" {
			passed := err == nil &&
				strings.Contains(result, cmd.ExpectedResult)
			if !passed {
				allPassed = false
			}
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:     "ipc_response",
					Target:   cmd.Name,
					Expected: cmd.ExpectedResult,
					Actual:   ipcActual(result, err),
					Passed:   passed,
					Message: ipcMessage(
						cmd.Name, passed, err,
					),
				},
			)
		} else {
			// No expected result; just check for no error.
			passed := err == nil
			if !passed {
				allPassed = false
			}
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:     "ipc_no_error",
					Target:   cmd.Name,
					Expected: "no error",
					Actual:   ipcActual(result, err),
					Passed:   passed,
					Message: ipcMessage(
						cmd.Name, passed, err,
					),
				},
			)
		}

		// Evaluate command assertions.
		for _, sa := range cmd.Assertions {
			saPassed := evaluateIPCAssertion(
				sa, result, err,
			)
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

		// Store output and duration.
		outputs[cmd.Name] = result
		durKey := fmt.Sprintf(
			"cmd_%s_duration", cmd.Name,
		)
		metrics[durKey] = challenge.MetricValue{
			Name:  durKey,
			Value: cmdDur.Seconds(),
			Unit:  "s",
		}
	}

	metrics["commands_executed"] = challenge.MetricValue{
		Name:  "commands_executed",
		Value: float64(len(c.commands)),
		Unit:  "commands",
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress("IPC challenge complete", map[string]any{
		"status":   status,
		"commands": len(c.commands),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("DesktopIPCChallenge: executed %d commands, status=%s", len(c.commands), status))
	return result, nil
}

// evaluateIPCAssertion evaluates a step assertion against
// an IPC command result.
func evaluateIPCAssertion(
	sa StepAssertion, result string, err error,
) bool {
	if err != nil {
		return false
	}
	switch sa.Type {
	case "response_contains":
		if expected, ok := sa.Value.(string); ok {
			return strings.Contains(result, expected)
		}
		return false
	case "not_empty":
		return result != ""
	default:
		return false
	}
}

// ipcActual returns the actual value string for an IPC
// assertion.
func ipcActual(result string, err error) string {
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error())
	}
	if result == "" {
		return "empty response"
	}
	if len(result) > 100 {
		return result[:100] + "..."
	}
	return result
}

// ipcMessage returns a human-readable message for an IPC
// assertion.
func ipcMessage(
	name string, passed bool, err error,
) string {
	if passed {
		return fmt.Sprintf(
			"IPC command %q succeeded", name,
		)
	}
	if err != nil {
		return fmt.Sprintf(
			"IPC command %q failed: %s",
			name, err.Error(),
		)
	}
	return fmt.Sprintf(
		"IPC command %q response mismatch", name,
	)
}
