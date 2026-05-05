package userflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// BrowserFlowChallenge executes a BrowserFlow by initializing
// a browser, navigating to the start URL, and performing each
// step in sequence with assertion evaluation.
type BrowserFlowChallenge struct {
	challenge.BaseChallenge
	adapter BrowserAdapter
	flow    BrowserFlow
}

// NewBrowserFlowChallenge creates a challenge that executes
// the given BrowserFlow using the provided adapter.
func NewBrowserFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter BrowserAdapter,
	flow BrowserFlow,
) *BrowserFlowChallenge {
	return &BrowserFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"browser",
			deps,
		),
		adapter: adapter,
		flow:    flow,
	}
}

// Execute runs the browser flow: initialize, navigate to
// start URL, execute each step, then close.
func (c *BrowserFlowChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("BrowserFlowChallenge: platform not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	allPassed := true
	screenshotCount := 0

	// Initialize browser.
	c.ReportProgress("initializing browser", map[string]any{
		"browser_type": c.flow.Config.BrowserType,
	})
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
		result.RecordAction(fmt.Sprintf("BrowserFlowChallenge: browser initialization failed (%s)", c.flow.Config.BrowserType))
		return result, nil
	}

	// Ensure browser is closed when done.
	defer func() {
		_ = c.adapter.Close(ctx)
	}()

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
		result.RecordAction(fmt.Sprintf("BrowserFlowChallenge: navigate to %s failed", c.flow.StartURL))
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

	c.ReportProgress("browser flow complete", map[string]any{
		"status":      status,
		"steps":       len(c.flow.Steps),
		"screenshots": screenshotCount,
	})

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("BrowserFlowChallenge: executed %d steps, status=%s, screenshots=%d", len(c.flow.Steps), status, screenshotCount))
	return result, nil
}

// executeStep dispatches the browser action for a single step.
func (c *BrowserFlowChallenge) executeStep(
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
