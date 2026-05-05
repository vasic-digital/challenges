package userflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// VisionFlowChallenge executes a browser flow using computer
// vision for element detection. Steps without a CSS selector
// use vision-based detection to find and interact with
// elements by type or text.
type VisionFlowChallenge struct {
	challenge.BaseChallenge
	browser BrowserAdapter
	vision  VisionAdapter
	flow    BrowserFlow
}

// NewVisionFlowChallenge creates a challenge that executes
// a browser flow using vision-based element detection for
// steps that lack CSS selectors.
func NewVisionFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	browser BrowserAdapter,
	vision VisionAdapter,
	flow BrowserFlow,
) *VisionFlowChallenge {
	return &VisionFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id), name, description,
			"browser", deps,
		),
		browser: browser,
		vision:  vision,
		flow:    flow,
	}
}

// Execute runs the vision-augmented browser flow: initialize,
// navigate, execute each step with optional vision detection,
// then close.
func (c *VisionFlowChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("VisionFlowChallenge: browser not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}
	if !c.vision.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "vision_available",
				Passed: true,
				Message: "Vision not available" +
					" - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("VisionFlowChallenge: vision not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	allPassed := true
	visionDetections := 0

	// Initialize browser.
	c.ReportProgress("initializing browser", map[string]any{
		"browser_type": c.flow.Config.BrowserType,
	})
	if err := c.browser.Initialize(
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
		result.RecordAction(fmt.Sprintf("VisionFlowChallenge: browser initialization failed (%s)", c.flow.Config.BrowserType))
		return result, nil
	}

	// Ensure browser is closed when done.
	defer func() {
		_ = c.browser.Close(ctx)
	}()

	// Navigate to start URL.
	c.ReportProgress(
		"navigating to start URL", map[string]any{
			"url": c.flow.StartURL,
		},
	)
	if err := c.browser.Navigate(
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
		result.RecordAction(fmt.Sprintf("VisionFlowChallenge: navigate to %s failed", c.flow.StartURL))
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
		var stepErr error

		if step.Selector != "" {
			// Standard browser action with CSS selector.
			stepErr = c.executeStep(ctx, step)
		} else {
			// Vision-based detection.
			stepErr = c.executeVisionStep(
				ctx, step, metrics,
				&visionDetections,
			)
		}

		stepDur := time.Since(stepStart)

		if stepErr != nil {
			allPassed = false
			assertions = append(
				assertions,
				challenge.AssertionResult{
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
				assertions,
				challenge.AssertionResult{
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

		// Evaluate step assertions.
		for _, sa := range step.Assertions {
			saPassed := stepErr == nil
			if !saPassed {
				allPassed = false
			}
			assertions = append(
				assertions,
				challenge.AssertionResult{
					Type:    sa.Type,
					Target:  sa.Target,
					Passed:  saPassed,
					Message: sa.Message,
				},
			)
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
	metrics["vision_elements_detected"] = challenge.MetricValue{
		Name:  "vision_elements_detected",
		Value: float64(visionDetections),
		Unit:  "elements",
	}

	outputs["vision_detections"] = fmt.Sprintf(
		"%d", visionDetections,
	)

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
	}

	c.ReportProgress(
		"vision flow complete", map[string]any{
			"status":            status,
			"steps":             len(c.flow.Steps),
			"vision_detections": visionDetections,
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("VisionFlowChallenge: executed %d steps, status=%s, vision_detections=%d", len(c.flow.Steps), status, visionDetections))
	return result, nil
}

// executeStep dispatches a standard browser action using a
// CSS selector.
func (c *VisionFlowChallenge) executeStep(
	ctx context.Context, step BrowserStep,
) error {
	switch step.Action {
	case "navigate":
		return c.browser.Navigate(ctx, step.Value)

	case "click":
		return c.browser.Click(ctx, step.Selector)

	case "fill":
		return c.browser.Fill(
			ctx, step.Selector, step.Value,
		)

	case "select":
		return c.browser.SelectOption(
			ctx, step.Selector, step.Value,
		)

	case "wait":
		timeout := step.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		return c.browser.WaitForSelector(
			ctx, step.Selector, timeout,
		)

	case "assert_visible":
		visible, err := c.browser.IsVisible(
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
		text, err := c.browser.GetText(
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

	case "screenshot":
		_, err := c.browser.Screenshot(ctx)
		return err

	case "evaluate_js":
		script := step.Value
		if step.Script != "" {
			script = step.Script
		}
		_, err := c.browser.EvaluateJS(ctx, script)
		return err

	default:
		return fmt.Errorf(
			"unknown browser action: %s", step.Action,
		)
	}
}

// executeVisionStep uses vision detection to locate an
// element and interact with it. If Value contains ":" it is
// parsed as "elemType:text" for FindByType + text filtering.
// Otherwise FindByText is used directly.
func (c *VisionFlowChallenge) executeVisionStep(
	ctx context.Context,
	step BrowserStep,
	metrics map[string]challenge.MetricValue,
	detections *int,
) error {
	screenshot, err := c.browser.Screenshot(ctx)
	if err != nil {
		return fmt.Errorf(
			"vision screenshot failed: %w", err,
		)
	}

	var found []DetectedElement

	if strings.Contains(step.Value, ":") {
		// Format "elemType:text".
		parts := strings.SplitN(step.Value, ":", 2)
		elemType := parts[0]
		text := parts[1]

		candidates, fErr := c.vision.FindByType(
			ctx, screenshot, elemType,
		)
		if fErr != nil {
			return fmt.Errorf(
				"vision FindByType failed: %w", fErr,
			)
		}

		lower := strings.ToLower(text)
		for _, e := range candidates {
			if strings.Contains(
				strings.ToLower(e.Text), lower,
			) {
				found = append(found, e)
			}
		}
	} else {
		found, err = c.vision.FindByText(
			ctx, screenshot, step.Value,
		)
		if err != nil {
			return fmt.Errorf(
				"vision FindByText failed: %w", err,
			)
		}
	}

	if len(found) == 0 {
		return fmt.Errorf(
			"vision: no element found for %q",
			step.Value,
		)
	}

	elem := found[0]
	*detections += len(found)

	// Record confidence metric for this step.
	confKey := fmt.Sprintf(
		"vision_confidence_%s", step.Name,
	)
	metrics[confKey] = challenge.MetricValue{
		Name:  confKey,
		Value: elem.Confidence,
		Unit:  "ratio",
	}

	// Click at the detected element's coordinates.
	clickJS := fmt.Sprintf(
		"document.elementFromPoint(%d,%d).click()",
		elem.Position.X, elem.Position.Y,
	)
	_, err = c.browser.EvaluateJS(ctx, clickJS)
	if err != nil {
		return fmt.Errorf(
			"vision click failed: %w", err,
		)
	}

	return nil
}
