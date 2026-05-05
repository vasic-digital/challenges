package userflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// RecordedAITestGenChallenge wraps AI test generation with
// video recording. It navigates to a target URL, records the
// session, captures a screenshot, generates test cases via
// AI analysis, verifies recording integrity, and saves the
// generated tests.
type RecordedAITestGenChallenge struct {
	challenge.BaseChallenge
	browser   BrowserAdapter
	recorder  RecorderAdapter
	testgen   TestGenAdapter
	targetURL string
	maxTests  int
	outputDir string
}

// NewRecordedAITestGenChallenge creates a challenge that
// records the browser session while generating tests via AI
// analysis of the target URL.
func NewRecordedAITestGenChallenge(
	id, name, description string,
	deps []challenge.ID,
	browser BrowserAdapter,
	recorder RecorderAdapter,
	testgen TestGenAdapter,
	targetURL string,
	maxTests int,
	outputDir string,
) *RecordedAITestGenChallenge {
	return &RecordedAITestGenChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id), name, description,
			"ai", deps,
		),
		browser:   browser,
		recorder:  recorder,
		testgen:   testgen,
		targetURL: targetURL,
		maxTests:  maxTests,
		outputDir: outputDir,
	}
}

// Execute runs the recorded AI test generation flow:
// check availability, initialize browser, start recording,
// navigate to target, capture screenshot, generate tests,
// stop recording, verify integrity, save results.
func (c *RecordedAITestGenChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check browser adapter availability.
	if !c.browser.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "browser_available",
				Passed: false,
				Message: "Browser not available" +
					" - skipped",
			}},
			nil, nil, "browser not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: browser not available, skipped (url=%s)", c.targetURL))
		return result, nil
	}

	// Check recorder adapter availability.
	if !c.recorder.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "recorder_available",
				Passed: false,
				Message: "Recorder not available" +
					" - skipped",
			}},
			nil, nil, "recorder not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: recorder not available, skipped (url=%s)", c.targetURL))
		return result, nil
	}

	// Check testgen adapter availability.
	if !c.testgen.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusSkipped, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "testgen_available",
				Passed: false,
				Message: "TestGen not available" +
					" - skipped",
			}},
			nil, nil, "testgen not available",
		)
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: testgen not available, skipped (url=%s)", c.targetURL))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)

	// Initialize browser.
	c.ReportProgress(
		"initializing browser", map[string]any{
			"target_url": c.targetURL,
		},
	)
	if err := c.browser.Initialize(
		ctx, BrowserConfig{Headless: true},
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
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: browser initialization failed (url=%s)", c.targetURL))
		return result, nil
	}
	defer func() {
		_ = c.browser.Close(ctx)
	}()

	// Determine recording output directory.
	recOutputDir := c.outputDir
	if recOutputDir == "" {
		recOutputDir = "/tmp"
	}

	// Start recording.
	recCfg := RecordingConfig{
		URL:       c.targetURL,
		OutputDir: recOutputDir,
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
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: recording start failed (url=%s)", c.targetURL))
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

	// Navigate to target URL.
	c.ReportProgress(
		"navigating to target", map[string]any{
			"url": c.targetURL,
		},
	)
	if err := c.browser.Navigate(
		ctx, c.targetURL,
	); err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "navigate",
				Target: "target_url",
				Passed: false,
				Message: fmt.Sprintf(
					"navigate to %s failed: %s",
					c.targetURL, err.Error(),
				),
			},
		)
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: navigate to %s failed", c.targetURL))
		return result, nil
	}

	// Take screenshot.
	c.ReportProgress(
		"capturing screenshot", map[string]any{
			"url": c.targetURL,
		},
	)
	screenshot, err := c.browser.Screenshot(ctx)
	if err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "screenshot",
				Target: "capture",
				Passed: false,
				Message: fmt.Sprintf(
					"screenshot failed: %s",
					err.Error(),
				),
			},
		)
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: screenshot failed (url=%s)", c.targetURL))
		return result, nil
	}

	// Generate tests via AI.
	c.ReportProgress(
		"generating tests via AI", map[string]any{
			"max_tests": c.maxTests,
		},
	)
	tests, err := c.testgen.GenerateTests(
		ctx, screenshot,
	)
	if err != nil {
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:   "testgen",
				Target: "generate_tests",
				Passed: false,
				Message: fmt.Sprintf(
					"test generation failed: %s",
					err.Error(),
				),
			},
		)
		_, _ = c.recorder.StopRecording(ctx)
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: test generation failed (url=%s)", c.targetURL))
		return result, nil
	}

	// Cap to maxTests if needed.
	if c.maxTests > 0 && len(tests) > c.maxTests {
		tests = tests[:c.maxTests]
	}

	// Compute unique categories.
	categories := make(map[string]bool)
	for _, t := range tests {
		if t.Category != "" {
			categories[t.Category] = true
		}
	}

	// Compute average confidence as coverage proxy.
	var totalConf float64
	for _, t := range tests {
		totalConf += t.Confidence
	}
	avgConf := 0.0
	if len(tests) > 0 {
		avgConf = totalConf / float64(len(tests))
	}

	// Stop recording and collect results.
	recResult, recErr := c.recorder.StopRecording(ctx)

	allPassed := true

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

	// Record test generation metrics.
	metrics["tests_generated"] = challenge.MetricValue{
		Name:  "tests_generated",
		Value: float64(len(tests)),
		Unit:  "tests",
	}
	metrics["test_categories"] = challenge.MetricValue{
		Name:  "test_categories",
		Value: float64(len(categories)),
		Unit:  "categories",
	}
	metrics["test_coverage"] = challenge.MetricValue{
		Name:  "test_coverage",
		Value: avgConf,
		Unit:  "ratio",
	}

	totalDur := time.Since(start)
	metrics["total_duration"] = challenge.MetricValue{
		Name:  "total_duration",
		Value: totalDur.Seconds(),
		Unit:  "s",
	}

	// Save generated tests to output directory.
	if c.outputDir != "" {
		if mkErr := os.MkdirAll(
			c.outputDir, 0o755,
		); mkErr != nil {
			assertions = append(
				assertions,
				challenge.AssertionResult{
					Type:   "output",
					Target: "create_dir",
					Passed: false,
					Message: fmt.Sprintf(
						"create output dir failed: %s",
						mkErr.Error(),
					),
				},
			)
			result := c.CreateResult(
				challenge.StatusFailed, start,
				assertions, metrics, outputs,
				mkErr.Error(),
			)
			result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: create output dir failed (%s)", c.outputDir))
			return result, nil
		}

		data, mErr := json.MarshalIndent(
			tests, "", "  ",
		)
		if mErr != nil {
			assertions = append(
				assertions,
				challenge.AssertionResult{
					Type:   "output",
					Target: "marshal_json",
					Passed: false,
					Message: fmt.Sprintf(
						"JSON marshal failed: %s",
						mErr.Error(),
					),
				},
			)
			result := c.CreateResult(
				challenge.StatusFailed, start,
				assertions, metrics, outputs,
				mErr.Error(),
			)
			result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: JSON marshal failed (%s)", c.outputDir))
			return result, nil
		}

		outPath := filepath.Join(
			c.outputDir, "generated_tests.json",
		)
		if wErr := os.WriteFile(
			outPath, data, 0o644,
		); wErr != nil {
			assertions = append(
				assertions,
				challenge.AssertionResult{
					Type:   "output",
					Target: "write_file",
					Passed: false,
					Message: fmt.Sprintf(
						"write file failed: %s",
						wErr.Error(),
					),
				},
			)
			result := c.CreateResult(
				challenge.StatusFailed, start,
				assertions, metrics, outputs,
				wErr.Error(),
			)
			result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: write file failed (%s)", outPath))
			return result, nil
		}

		outputs["output_file"] = outPath
	}

	outputs["tests_generated"] = fmt.Sprintf(
		"%d", len(tests),
	)
	outputs["test_categories"] = fmt.Sprintf(
		"%d", len(categories),
	)

	// Build success assertions for test generation.
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:   "testgen",
			Target: "tests_generated",
			Passed: len(tests) > 0,
			Message: fmt.Sprintf(
				"generated %d test(s)", len(tests),
			),
		},
	)
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:   "testgen",
			Target: "test_categories",
			Passed: len(categories) > 0,
			Message: fmt.Sprintf(
				"%d unique categor(ies)",
				len(categories),
			),
		},
	)

	// Determine final status.
	for _, a := range assertions {
		if !a.Passed {
			allPassed = false
			break
		}
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
		"recorded AI test generation complete",
		map[string]any{
			"status":     status,
			"tests":      len(tests),
			"categories": len(categories),
			"coverage":   avgConf,
			"recorded":   recResult != nil,
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs,
		errMsg,
	)
	result.RecordAction(fmt.Sprintf("RecordedAITestGenChallenge: generated %d tests, categories=%d, status=%s", len(tests), len(categories), status))
	return result, nil
}
