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

// AITestGenerationChallenge navigates to a target URL, takes
// a screenshot, and generates test cases using AI analysis.
type AITestGenerationChallenge struct {
	challenge.BaseChallenge
	browser   BrowserAdapter
	testgen   TestGenAdapter
	targetURL string
	maxTests  int
	outputDir string
}

// NewAITestGenerationChallenge creates a challenge that
// navigates to a URL, captures a screenshot, and generates
// test cases via AI analysis.
func NewAITestGenerationChallenge(
	id, name, description string,
	deps []challenge.ID,
	browser BrowserAdapter,
	testgen TestGenAdapter,
	targetURL string,
	maxTests int,
	outputDir string,
) *AITestGenerationChallenge {
	return &AITestGenerationChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id), name, description,
			"ai", deps,
		),
		browser:   browser,
		testgen:   testgen,
		targetURL: targetURL,
		maxTests:  maxTests,
		outputDir: outputDir,
	}
}

// Execute runs the AI test generation flow: initialize
// browser, navigate to target, capture screenshot, generate
// tests, save results.
func (c *AITestGenerationChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check infrastructure availability.
	if !c.browser.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "browser_available",
				Passed: true,
				Message: "Browser not available" +
					" - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: browser not available, skipped (url=%s)", c.targetURL))
		return result, nil
	}
	if !c.testgen.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "testgen_available",
				Passed: true,
				Message: "TestGen not available" +
					" - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: testgen not available, skipped (url=%s)", c.targetURL))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)

	// Initialize browser.
	c.ReportProgress("initializing browser", map[string]any{
		"target_url": c.targetURL,
	})
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
		result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: browser initialization failed (url=%s)", c.targetURL))
		return result, nil
	}
	defer func() {
		_ = c.browser.Close(ctx)
	}()

	// Navigate to target URL.
	c.ReportProgress("navigating to target", map[string]any{
		"url": c.targetURL,
	})
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
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: navigate to %s failed", c.targetURL))
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
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: screenshot failed (url=%s)", c.targetURL))
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
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			err.Error(),
		)
		result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: test generation failed (url=%s)", c.targetURL))
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

	// Record metrics.
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
			result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: create output dir failed (%s)", c.outputDir))
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
			result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: JSON marshal failed (%s)", c.outputDir))
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
			result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: write file failed (%s)", outPath))
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

	// Build success assertions.
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

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	c.ReportProgress(
		"AI test generation complete", map[string]any{
			"status":     status,
			"tests":      len(tests),
			"categories": len(categories),
			"coverage":   avgConf,
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("AITestGenerationChallenge: generated %d tests, categories=%d, status=%s", len(tests), len(categories), status))
	return result, nil
}
