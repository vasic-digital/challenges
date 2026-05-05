package userflow

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// RecordedMobileLaunchChallenge installs, launches, and
// verifies stability of a mobile application while recording
// the entire session as video. The recording is verified for
// integrity (non-zero file size, duration, and frame count).
type RecordedMobileLaunchChallenge struct {
	challenge.BaseChallenge
	adapter       MobileAdapter
	recorder      RecorderAdapter
	appPath       string
	stabilityWait time.Duration
}

// NewRecordedMobileLaunchChallenge creates a challenge that
// installs and launches the app from appPath with video
// recording, then waits stabilityWait to verify it remains
// running.
func NewRecordedMobileLaunchChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter MobileAdapter,
	recorder RecorderAdapter,
	appPath string,
	stabilityWait time.Duration,
) *RecordedMobileLaunchChallenge {
	return &RecordedMobileLaunchChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"mobile",
			deps,
		),
		adapter:       adapter,
		recorder:      recorder,
		appPath:       appPath,
		stabilityWait: stabilityWait,
	}
}

// Execute installs the app, launches it with video recording,
// waits for the stability period, checks if it is still
// running, takes a screenshot, stops the app, and verifies
// recording integrity.
func (c *RecordedMobileLaunchChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check adapter availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "platform_available",
				Passed: false,
				Message: "Mobile adapter not " +
					"available - skipped",
			}},
			nil, nil,
			"mobile adapter not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: mobile adapter not available, skipped (app=%s)", c.appPath))
		return result, nil
	}

	// Check recorder availability.
	if !c.recorder.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "recorder_available",
				Passed: false,
				Message: "Recorder not " +
					"available - skipped",
			}},
			nil, nil,
			"recorder not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: recorder not available, skipped (app=%s)", c.appPath))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)

	// Start recording.
	c.ReportProgress(
		"starting recording for mobile launch",
		map[string]any{"app_path": c.appPath},
	)
	recCfg := RecordingConfig{
		URL:       "mobile://launch",
		OutputDir: "mobile",
	}
	if err := c.recorder.StartRecording(
		ctx, recCfg,
	); err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "recording_start",
				Target: "start_recording",
				Passed: false,
				Message: fmt.Sprintf(
					"start recording failed: %s",
					err.Error(),
				),
			},
		)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: recording start failed (app=%s)", c.appPath))
		return result, nil
	}

	assertions = append(
		assertions, challenge.AssertionResult{
			Type:    "recording_start",
			Target:  "start_recording",
			Passed:  true,
			Message: "recording started successfully",
		},
	)

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
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			installErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: install failed for %s", c.appPath))
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
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			launchErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: launch failed for %s", c.appPath))
		return result, nil
	}

	// Wait for stability.
	c.ReportProgress(
		"waiting for stability", map[string]any{
			"duration": c.stabilityWait.String(),
		},
	)
	select {
	case <-ctx.Done():
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			ctx.Err().Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: context cancelled during stability wait for %s", c.appPath))
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
			Message: stabilityMessage(
				running, runErr,
			),
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

	// Stop recording and collect results.
	recResult, recErr := c.recorder.StopRecording(ctx)

	allPassed := true
	for _, a := range assertions {
		if !a.Passed {
			allPassed = false
			break
		}
	}

	// Add recording metrics and verify integrity.
	if recResult != nil {
		metrics["video_duration"] = challenge.MetricValue{
			Name:  "video_duration",
			Value: recResult.Duration.Seconds(),
			Unit:  "s",
		}
		metrics["video_frame_count"] = challenge.MetricValue{
			Name:  "video_frame_count",
			Value: float64(recResult.FrameCount),
			Unit:  "frames",
		}
		metrics["video_file_size"] = challenge.MetricValue{
			Name:  "video_file_size",
			Value: float64(recResult.FileSize),
			Unit:  "bytes",
		}
		outputs["video_path"] = recResult.FilePath

		// Verify recording integrity.
		integrity := recResult.FileSize > 0 &&
			recResult.Duration > 0 &&
			recResult.FrameCount > 0
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "recording",
				Target: "video_recorded",
				Passed: recResult != nil,
				Message: "video recording captured " +
					"successfully",
			},
			challenge.AssertionResult{
				Type:   "recording",
				Target: "video_integrity",
				Passed: integrity,
				Message: fmt.Sprintf(
					"video integrity: size=%d "+
						"duration=%s frames=%d",
					recResult.FileSize,
					recResult.Duration,
					recResult.FrameCount,
				),
			},
		)
		if !integrity {
			allPassed = false
		}
	} else {
		allPassed = false
		errMsg := "recording returned nil result"
		if recErr != nil {
			errMsg = recErr.Error()
		}
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "recording",
				Target: "video_recorded",
				Passed: false,
				Message: fmt.Sprintf(
					"recording failed: %s", errMsg,
				),
			},
		)
	}

	status := challenge.StatusPassed
	errMsg := ""
	if !allPassed {
		status = challenge.StatusFailed
	}
	if recErr != nil {
		errMsg = recErr.Error()
	}

	result := c.CreateResult(
		status, start, assertions, metrics, outputs,
		errMsg,
	)
	result.RecordAction(fmt.Sprintf("RecordedMobileLaunchChallenge: launched %s, stable=%t, status=%s, recorded=%t", c.appPath, stablePassed, status, recResult != nil))
	return result, nil
}

// RecordedMobileFlowChallenge executes a MobileFlow by
// dispatching each step action to the MobileAdapter while
// recording the entire session as video. The recording is
// verified for integrity after the flow completes.
type RecordedMobileFlowChallenge struct {
	challenge.BaseChallenge
	adapter  MobileAdapter
	recorder RecorderAdapter
	flow     MobileFlow
}

// NewRecordedMobileFlowChallenge creates a challenge that
// records the mobile session while executing a flow.
func NewRecordedMobileFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter MobileAdapter,
	recorder RecorderAdapter,
	flow MobileFlow,
) *RecordedMobileFlowChallenge {
	return &RecordedMobileFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id), name, description,
			"mobile", deps,
		),
		adapter:  adapter,
		recorder: recorder,
		flow:     flow,
	}
}

// Execute runs each step in the mobile flow with video
// recording, collecting assertions and metrics per step,
// then verifies recording integrity.
func (c *RecordedMobileFlowChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check adapter availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "platform_available",
				Passed: false,
				Message: "Mobile adapter not " +
					"available - skipped",
			}},
			nil, nil,
			"mobile adapter not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileFlowChallenge: mobile adapter not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	// Check recorder availability.
	if !c.recorder.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "recorder_available",
				Passed: false,
				Message: "Recorder not " +
					"available - skipped",
			}},
			nil, nil,
			"recorder not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileFlowChallenge: recorder not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	allPassed := true

	// Start recording.
	c.ReportProgress(
		"starting recording for mobile flow",
		map[string]any{"flow": c.flow.Name},
	)
	recCfg := RecordingConfig{
		URL:       "mobile://flow",
		OutputDir: "mobile",
	}
	if err := c.recorder.StartRecording(
		ctx, recCfg,
	); err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "recording_start",
				Target: "start_recording",
				Passed: false,
				Message: fmt.Sprintf(
					"start recording failed: %s",
					err.Error(),
				),
			},
		)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedMobileFlowChallenge: recording start failed (flow=%s)", c.flow.Name))
		return result, nil
	}

	assertions = append(
		assertions, challenge.AssertionResult{
			Type:    "recording_start",
			Target:  "start_recording",
			Passed:  true,
			Message: "recording started successfully",
		},
	)

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

	// Stop recording and collect results.
	recResult, recErr := c.recorder.StopRecording(ctx)

	// Add recording metrics and verify integrity.
	if recResult != nil {
		metrics["video_duration"] = challenge.MetricValue{
			Name:  "video_duration",
			Value: recResult.Duration.Seconds(),
			Unit:  "s",
		}
		metrics["video_frame_count"] = challenge.MetricValue{
			Name:  "video_frame_count",
			Value: float64(recResult.FrameCount),
			Unit:  "frames",
		}
		metrics["video_file_size"] = challenge.MetricValue{
			Name:  "video_file_size",
			Value: float64(recResult.FileSize),
			Unit:  "bytes",
		}
		outputs["video_path"] = recResult.FilePath

		// Verify recording integrity.
		integrity := recResult.FileSize > 0 &&
			recResult.Duration > 0 &&
			recResult.FrameCount > 0
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "recording",
				Target: "video_recorded",
				Passed: recResult != nil,
				Message: "video recording captured " +
					"successfully",
			},
			challenge.AssertionResult{
				Type:   "recording",
				Target: "video_integrity",
				Passed: integrity,
				Message: fmt.Sprintf(
					"video integrity: size=%d "+
						"duration=%s frames=%d",
					recResult.FileSize,
					recResult.Duration,
					recResult.FrameCount,
				),
			},
		)
		if !integrity {
			allPassed = false
		}
	} else {
		allPassed = false
		errMsg := "recording returned nil result"
		if recErr != nil {
			errMsg = recErr.Error()
		}
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "recording",
				Target: "video_recorded",
				Passed: false,
				Message: fmt.Sprintf(
					"recording failed: %s", errMsg,
				),
			},
		)
	}

	status := challenge.StatusPassed
	errMsg := ""
	if !allPassed {
		status = challenge.StatusFailed
	}
	if recErr != nil {
		errMsg = recErr.Error()
	}

	c.ReportProgress(
		"recorded mobile flow complete",
		map[string]any{
			"status":   status,
			"steps":    len(c.flow.Steps),
			"recorded": recResult != nil,
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs,
		errMsg,
	)
	result.RecordAction(fmt.Sprintf("RecordedMobileFlowChallenge: executed %d steps, status=%s, recorded=%t", len(c.flow.Steps), status, recResult != nil))
	return result, nil
}

// executeStep dispatches the mobile action for a single step.
func (c *RecordedMobileFlowChallenge) executeStep(
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
