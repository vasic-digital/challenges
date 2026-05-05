package userflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// APIHealthChallenge performs a simple health endpoint check
// via an APIAdapter, asserting the expected HTTP status code.
type APIHealthChallenge struct {
	challenge.BaseChallenge
	adapter      APIAdapter
	healthPath   string
	expectedCode int
}

// NewAPIHealthChallenge creates a health-check challenge that
// GETs the given path and expects the given HTTP status code.
func NewAPIHealthChallenge(
	id string,
	adapter APIAdapter,
	healthPath string,
	expectedCode int,
	deps []challenge.ID,
) *APIHealthChallenge {
	return &APIHealthChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			"API Health Check",
			fmt.Sprintf(
				"Verify %s returns HTTP %d",
				healthPath, expectedCode,
			),
			"api",
			deps,
		),
		adapter:      adapter,
		healthPath:   healthPath,
		expectedCode: expectedCode,
	}
}

// Execute GETs the health path and asserts the status code.
func (c *APIHealthChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("APIHealthChallenge: platform not available, skipped (path=%s)", c.healthPath))
		return result, nil
	}

	c.ReportProgress("checking API health", map[string]any{
		"path": c.healthPath,
	})

	code, _, err := c.adapter.GetRaw(ctx, c.healthPath)

	passed := err == nil && code == c.expectedCode
	msg := healthAssertionMessage(
		c.healthPath, c.expectedCode, code, err,
	)

	assertions := []challenge.AssertionResult{
		{
			Type:   "status_code",
			Target: c.healthPath,
			Expected: fmt.Sprintf(
				"%d", c.expectedCode,
			),
			Actual:  fmt.Sprintf("%d", code),
			Passed:  passed,
			Message: msg,
		},
	}

	duration := time.Since(start)
	metrics := map[string]challenge.MetricValue{
		"response_time": {
			Name:  "response_time",
			Value: duration.Seconds(),
			Unit:  "s",
		},
	}

	status := challenge.StatusPassed
	var errMsg string
	if !passed {
		status = challenge.StatusFailed
		if err != nil {
			errMsg = err.Error()
		}
	}

	result := c.CreateResult(
		status, start, assertions, metrics, nil, errMsg,
	)
	result.RecordAction(fmt.Sprintf("APIHealthChallenge: health check %s returned HTTP %d (expected %d), status=%s", c.healthPath, code, c.expectedCode, status))
	return result, nil
}

// healthAssertionMessage returns a human-readable message for
// the health check assertion.
func healthAssertionMessage(
	path string, expected, actual int, err error,
) string {
	if err != nil {
		return fmt.Sprintf(
			"health check %s failed: %s",
			path, err.Error(),
		)
	}
	if actual == expected {
		return fmt.Sprintf(
			"health check %s returned %d as expected",
			path, actual,
		)
	}
	return fmt.Sprintf(
		"health check %s expected %d but got %d",
		path, expected, actual,
	)
}

// APIFlowChallenge executes a multi-step API flow, optionally
// logging in first, then performing each step in sequence with
// variable extraction and assertion evaluation.
type APIFlowChallenge struct {
	challenge.BaseChallenge
	adapter APIAdapter
	flow    APIFlow
}

// NewAPIFlowChallenge creates a challenge that executes the
// given APIFlow using the provided adapter.
func NewAPIFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter APIAdapter,
	flow APIFlow,
) *APIFlowChallenge {
	return &APIFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"api",
			deps,
		),
		adapter: adapter,
		flow:    flow,
	}
}

// Execute runs the full API flow: login, then each step in
// order with variable substitution and assertion checking.
func (c *APIFlowChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("APIFlowChallenge: platform not available, skipped (%d steps)", len(c.flow.Steps)))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	variables := make(map[string]string)
	allPassed := true

	// Login if credentials are provided.
	if c.flow.Credentials.Username != "" {
		c.ReportProgress("logging in", map[string]any{
			"user": c.flow.Credentials.Username,
		})
		token, err := c.adapter.LoginWithRetry(
			ctx, c.flow.Credentials, 5,
		)
		loginPassed := err == nil && token != ""
		if !loginPassed {
			allPassed = false
		}

		assertions = append(
			assertions, challenge.AssertionResult{
				Type:     "login",
				Target:   "auth_token",
				Expected: "non-empty token",
				Actual:   loginActual(token, err),
				Passed:   loginPassed,
				Message:  loginMessage(loginPassed, err),
			},
		)

		if loginPassed {
			c.adapter.SetToken(token)
			variables["token"] = token
		}
	}

	// Execute each step.
	for i, step := range c.flow.Steps {
		c.ReportProgress(
			fmt.Sprintf(
				"step %d/%d: %s",
				i+1, len(c.flow.Steps), step.Name,
			),
			map[string]any{"step": step.Name},
		)

		stepStart := time.Now()

		// Substitute variables in path and body.
		path := substituteVars(step.Path, variables)
		body := substituteVars(step.Body, variables)

		// Execute the HTTP method.
		code, respBody, err := c.executeStep(
			ctx, step.Method, path, body,
		)

		// Check status code if expected.
		if step.ExpectedStatus > 0 || len(step.AcceptedStatuses) > 0 {
			statusPassed := err == nil
			if statusPassed {
				if step.ExpectedStatus > 0 {
					statusPassed = code == step.ExpectedStatus
				}
				if !statusPassed && len(step.AcceptedStatuses) > 0 {
					for _, accepted := range step.AcceptedStatuses {
						if code == accepted {
							statusPassed = true
							break
						}
					}
				}
			}
			if !statusPassed {
				allPassed = false
			}
			expectedStr := fmt.Sprintf("%d", step.ExpectedStatus)
			if len(step.AcceptedStatuses) > 0 {
				expectedStr = fmt.Sprintf("%v", step.AcceptedStatuses)
			}
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:     "status_code",
					Target:   step.Name,
					Expected: expectedStr,
					Actual:   fmt.Sprintf("%d", code),
					Passed:   statusPassed,
					Message: stepStatusMessage(
						step.Name,
						step.ExpectedStatus,
						code, err,
					),
				},
			)
		}

		// Extract variables from response.
		if len(step.ExtractTo) > 0 && respBody != nil {
			// Try to parse as object first
			var respMap map[string]interface{}
			if jsonErr := json.Unmarshal(
				respBody, &respMap,
			); jsonErr == nil {
				for field, varName := range step.ExtractTo {
					if val, ok := respMap[field]; ok {
						variables[varName] = fmt.Sprintf(
							"%v", val,
						)
					}
				}
			} else {
				// Try to parse as array
				var respArray []interface{}
				if jsonErr := json.Unmarshal(
					respBody, &respArray,
				); jsonErr == nil && len(respArray) > 0 {
					for field, varName := range step.ExtractTo {
						// Handle "[0].id" syntax
						if len(field) > 3 && field[0] == '[' {
							idx := int(field[1] - '0')
							if idx < len(respArray) {
								if obj, ok := respArray[idx].(map[string]interface{}); ok {
									key := field[4:] // Skip "[0]."
									if val, ok := obj[key]; ok {
										variables[varName] = fmt.Sprintf("%v", val)
									}
								}
							}
						}
					}
				}
			}
		}

		// Evaluate step assertions.
		for _, sa := range step.Assertions {
			saPassed := evaluateStepAssertion(
				sa, code, respBody, err,
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

		// Record per-step duration.
		stepDur := time.Since(stepStart)
		durKey := fmt.Sprintf(
			"step_%s_duration", step.Name,
		)
		metrics[durKey] = challenge.MetricValue{
			Name:  durKey,
			Value: stepDur.Seconds(),
			Unit:  "s",
		}

		// Store response body.
		if respBody != nil {
			outputs[step.Name] = string(respBody)
		}
	}

	status := challenge.StatusPassed
	if !allPassed {
		status = challenge.StatusFailed
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

	c.ReportProgress("API flow complete", map[string]any{
		"status": status,
		"steps":  len(c.flow.Steps),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("APIFlowChallenge: executed %d steps, status=%s", len(c.flow.Steps), status))
	return result, nil
}

// executeStep dispatches the HTTP method and returns the
// status code, response body, and any error.
func (c *APIFlowChallenge) executeStep(
	ctx context.Context,
	method, path, body string,
) (int, []byte, error) {
	switch strings.ToUpper(method) {
	case "GET":
		return c.adapter.GetRaw(ctx, path)
	case "POST":
		return c.adapter.PostJSON(ctx, path, body)
	case "PUT":
		return c.adapter.PutJSON(ctx, path, body)
	case "DELETE":
		if body != "" {
			return c.adapter.DeleteWithBody(ctx, path, body)
		}
		return c.adapter.Delete(ctx, path)
	default:
		return 0, nil, fmt.Errorf(
			"unsupported HTTP method: %s", method,
		)
	}
}

// substituteVars replaces {{var}} placeholders in s with
// values from the variables map.
func substituteVars(
	s string, variables map[string]string,
) string {
	for k, v := range variables {
		s = strings.ReplaceAll(
			s, "{{"+k+"}}", v,
		)
	}
	return s
}

// evaluateStepAssertion evaluates a single step assertion
// against the HTTP response.
func evaluateStepAssertion(
	sa StepAssertion,
	code int, body []byte, err error,
) bool {
	if err != nil {
		return false
	}
	switch sa.Type {
	case "status_code":
		if expected, ok := sa.Value.(float64); ok {
			return code == int(expected)
		}
		if expected, ok := sa.Value.(int); ok {
			return code == expected
		}
		return false
	case "response_contains":
		if expected, ok := sa.Value.(string); ok {
			return strings.Contains(
				string(body), expected,
			)
		}
		return false
	case "not_empty":
		return len(body) > 0
	default:
		// Unknown assertion types are treated as failed.
		return false
	}
}

// loginActual returns the actual value string for a login
// assertion.
func loginActual(token string, err error) string {
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error())
	}
	if token == "" {
		return "empty token"
	}
	return "token received"
}

// loginMessage returns a human-readable message for the login
// assertion.
func loginMessage(passed bool, err error) string {
	if passed {
		return "login succeeded"
	}
	if err != nil {
		return fmt.Sprintf("login failed: %s", err.Error())
	}
	return "login returned empty token"
}

// stepStatusMessage returns a human-readable message for a
// step status code assertion.
func stepStatusMessage(
	stepName string, expected, actual int, err error,
) string {
	if err != nil {
		return fmt.Sprintf(
			"step %q failed: %s",
			stepName, err.Error(),
		)
	}
	if actual == expected {
		return fmt.Sprintf(
			"step %q returned %d as expected",
			stepName, actual,
		)
	}
	return fmt.Sprintf(
		"step %q expected %d but got %d",
		stepName, expected, actual,
	)
}
