package userflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// RecordedBrowserFlowChallenge executes a browser flow while
// recording the entire session as video. The recording is
// verified for integrity (non-zero file size, duration, and
// frame count).
type RecordedBrowserFlowChallenge struct {
	challenge.BaseChallenge
	adapter  BrowserAdapter
	recorder RecorderAdapter
	flow     BrowserFlow
}

// NewRecordedBrowserFlowChallenge creates a challenge that
// records the browser session while executing a flow.
func NewRecordedBrowserFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter BrowserAdapter,
	recorder RecorderAdapter,
	flow BrowserFlow,
) *RecordedBrowserFlowChallenge {
	return &RecordedBrowserFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id), name, description,
			"browser", deps,
		),
		adapter:  adapter,
		recorder: recorder,
		flow:     flow,
	}
}

// Execute runs the browser flow with video recording:
// initialize browser and recorder, navigate to start URL,
// execute each step, stop recording, then close.
func (c *RecordedBrowserFlowChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check browser adapter availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:    "infrastructure",
				Target:  "browser_available",
				Passed:  false,
				Message: "Browser not available - skipped",
			}},
			nil, nil, "browser not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedBrowserFlowChallenge: browser not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	// Check recorder adapter availability.
	if !c.recorder.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:    "infrastructure",
				Target:  "recorder_available",
				Passed:  false,
				Message: "Recorder not available - skipped",
			}},
			nil, nil, "recorder not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedBrowserFlowChallenge: recorder not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	allPassed := true
	screenshotCount := 0

	// Initialize browser and recorder.
	c.ReportProgress(
		"initializing browser and recorder",
		map[string]any{
			"browser_type": c.flow.Config.BrowserType,
		},
	)

	if err := c.adapter.Initialize(
		ctx, c.flow.Config,
	); err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "browser_init",
				Target: "initialize",
				Passed: false,
				Message: fmt.Sprintf(
					"browser init failed: %s",
					err.Error(),
				),
			},
		)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedBrowserFlowChallenge: browser initialization failed (%s)", c.flow.Config.BrowserType))
		return result, nil
	}

	// Ensure browser is closed when done.
	defer func() {
		_ = c.adapter.Close(ctx)
	}()

	// Determine output directory for the recording.
	outputDir := c.flow.Config.BrowserType
	if outputDir == "" {
		outputDir = "/tmp"
	}

	// Start recording.
	recCfg := RecordingConfig{
		URL:       c.flow.StartURL,
		OutputDir: outputDir,
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
		result.RecordAction(fmt.Sprintf("RecordedBrowserFlowChallenge: recording start failed (url=%s)", c.flow.StartURL))
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
		// Stop recording before returning on failure.
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedBrowserFlowChallenge: navigate to %s failed", c.flow.StartURL))
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

		// Take screenshot after step if requested.
		if step.Screenshot && stepErr == nil {
			if _, ssErr := c.adapter.Screenshot(
				ctx,
			); ssErr == nil {
				screenshotCount++
			}
		}

		// Record step duration.
		durKey := fmt.Sprintf(
			"step_%s_duration", step.Name,
		)
		metrics[durKey] = challenge.MetricValue{
			Name:  durKey,
			Value: stepDur.Seconds(),
			Unit:  "s",
		}
	}

	// Stop recording and collect results.
	recResult, recErr := c.recorder.StopRecording(ctx)

	// Add recording metrics.
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
	errMsg := ""
	if !allPassed {
		status = challenge.StatusFailed
	}
	if recErr != nil {
		errMsg = recErr.Error()
	}

	c.ReportProgress(
		"recorded browser flow complete",
		map[string]any{
			"status":      status,
			"steps":       len(c.flow.Steps),
			"screenshots": screenshotCount,
			"recorded":    recResult != nil,
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs,
		errMsg,
	)
	result.RecordAction(fmt.Sprintf("RecordedBrowserFlowChallenge: executed %d steps, status=%s, recorded=%t", len(c.flow.Steps), status, recResult != nil))
	return result, nil
}

// executeStep dispatches the browser action for a single step.
func (c *RecordedBrowserFlowChallenge) executeStep(
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

	case "select":
		return c.adapter.SelectOption(
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

	case "assert_text":
		text, err := c.adapter.GetText(
			ctx, step.Selector,
		)
		if err != nil {
			return err
		}
		if !strings.Contains(text, step.Value) {
			return fmt.Errorf(
				"text %q not found in element %q "+
					"(got %q)",
				step.Value, step.Selector, text,
			)
		}
		return nil

	case "assert_url":
		url, err := c.adapter.EvaluateJS(
			ctx, "window.location.href",
		)
		if err != nil {
			return err
		}
		if !strings.Contains(url, step.Value) {
			return fmt.Errorf(
				"URL does not contain %q (got %q)",
				step.Value, url,
			)
		}
		return nil

	case "screenshot":
		_, err := c.adapter.Screenshot(ctx)
		return err

	case "evaluate_js":
		script := step.Value
		if step.Script != "" {
			script = step.Script
		}
		_, err := c.adapter.EvaluateJS(ctx, script)
		return err

	default:
		return fmt.Errorf(
			"unknown browser action: %s", step.Action,
		)
	}
}
