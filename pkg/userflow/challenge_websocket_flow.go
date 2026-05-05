package userflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// Compile-time interface check.
var _ challenge.Challenge = (*WebSocketFlowChallenge)(nil)

// WebSocketFlow defines a sequence of WebSocket steps to
// execute as a user flow test.
type WebSocketFlow struct {
	// URL is the WebSocket endpoint URL (ws:// or wss://).
	URL string `json:"url"`

	// Headers are sent during the WebSocket handshake.
	Headers map[string]string `json:"headers,omitempty"`

	// Steps is the ordered sequence of WebSocket steps.
	Steps []WebSocketStep `json:"steps"`
}

// WebSocketStep defines a single step in a WebSocket flow.
type WebSocketStep struct {
	// Name identifies this step.
	Name string `json:"name"`

	// Action is the step type: "send", "receive",
	// "send_receive", "receive_all", or "wait".
	Action string `json:"action"`

	// Message is the payload for send actions.
	Message string `json:"message,omitempty"`

	// Timeout is the maximum wait duration for receive
	// actions and the sleep duration for wait actions.
	Timeout time.Duration `json:"timeout,omitempty"`

	// Assertions define checks to run on the result.
	Assertions []StepAssertion `json:"assertions"`

	// ExtractTo maps response JSON field names to variable
	// names. Extracted values can be referenced in subsequent
	// steps via {{var_name}} placeholders.
	ExtractTo map[string]string `json:"extract_to,omitempty"`
}

// WebSocketFlowChallenge executes a multi-step WebSocket
// flow using a WebSocketFlowAdapter. It connects to the
// endpoint, then iterates through steps performing send,
// receive, and assertion operations.
type WebSocketFlowChallenge struct {
	challenge.BaseChallenge
	adapter WebSocketFlowAdapter
	flow    WebSocketFlow
}

// NewWebSocketFlowChallenge creates a challenge that executes
// the given WebSocketFlow using the provided adapter.
func NewWebSocketFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter WebSocketFlowAdapter,
	flow WebSocketFlow,
) *WebSocketFlowChallenge {
	return &WebSocketFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"websocket",
			deps,
		),
		adapter: adapter,
		flow:    flow,
	}
}

// Execute runs the full WebSocket flow: connects to the
// endpoint, then executes each step in order with variable
// substitution and assertion checking.
func (c *WebSocketFlowChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// Check infrastructure availability.
	if !c.adapter.Available(ctx) {
		result := c.CreateResult(
			challenge.StatusPassed, start,
			[]challenge.AssertionResult{{
				Type:   "infrastructure",
				Target: "platform_available",
				Passed: true,
				Message: "Platform not available" +
					" - skipped (requires infrastructure)",
			}},
			nil, nil, "",
		)
		result.RecordAction(fmt.Sprintf("WebSocketFlowChallenge: platform not available, skipped (%d steps, url=%s)", len(c.flow.Steps), c.flow.URL))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	variables := make(map[string]string)
	allPassed := true

	// Connect to WebSocket endpoint.
	c.ReportProgress("connecting to WebSocket", map[string]any{
		"url": c.flow.URL,
	})
	connErr := c.adapter.Connect(
		ctx, c.flow.URL, c.flow.Headers,
	)
	connPassed := connErr == nil
	if !connPassed {
		allPassed = false
	}
	assertions = append(
		assertions, challenge.AssertionResult{
			Type:     "ws_connect",
			Target:   "connection",
			Expected: "connected",
			Actual:   wsConnectActual(connErr),
			Passed:   connPassed,
			Message:  wsConnectMessage(connErr),
		},
	)
	if connErr != nil {
		result := c.CreateResult(
			challenge.StatusFailed, start,
			assertions, metrics, outputs,
			connErr.Error(),
		)
		result.RecordAction(fmt.Sprintf("WebSocketFlowChallenge: connection to %s failed", c.flow.URL))
		return result, nil
	}

	// Ensure connection is closed when done.
	defer func() {
		_ = c.adapter.Close(ctx)
	}()

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

		// Substitute variables in message.
		message := substituteVars(step.Message, variables)

		// Execute the step action.
		var response []byte
		var allResponses [][]byte
		var stepErr error

		timeout := step.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		switch step.Action {
		case "send":
			stepErr = c.adapter.Send(
				ctx, []byte(message),
			)
		case "receive":
			response, stepErr = c.adapter.Receive(
				ctx, timeout,
			)
		case "send_receive":
			response, stepErr = c.adapter.SendAndReceive(
				ctx, []byte(message), timeout,
			)
		case "receive_all":
			allResponses, stepErr = c.adapter.ReceiveAll(
				ctx, timeout,
			)
			if stepErr == nil && len(allResponses) > 0 {
				response = allResponses[0]
			}
		case "wait":
			select {
			case <-ctx.Done():
				stepErr = ctx.Err()
			case <-time.After(timeout):
			}
		default:
			stepErr = fmt.Errorf(
				"unsupported WebSocket action: %s",
				step.Action,
			)
		}

		// Check step execution result.
		stepPassed := stepErr == nil
		if !stepPassed {
			allPassed = false
		}
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:     "ws_step",
				Target:   step.Name,
				Expected: "success",
				Actual:   wsStepActual(stepErr),
				Passed:   stepPassed,
				Message: wsStepMessage(
					step.Name, step.Action, stepErr,
				),
			},
		)

		// Extract variables from response.
		if stepErr == nil && len(step.ExtractTo) > 0 &&
			response != nil {
			extractWSVariables(
				response, step.ExtractTo, variables,
			)
		}

		// Evaluate step assertions.
		for _, sa := range step.Assertions {
			saPassed := evaluateWSStepAssertion(
				sa, response, allResponses, stepErr,
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

		// Store response output.
		if response != nil {
			outputs[step.Name] = string(response)
		}
		if len(allResponses) > 0 {
			strs := make([]string, len(allResponses))
			for j, r := range allResponses {
				strs[j] = string(r)
			}
			data, jsonErr := json.Marshal(strs)
			if jsonErr == nil {
				outputs[step.Name+"_all"] = string(data)
			}
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

	c.ReportProgress(
		"WebSocket flow complete", map[string]any{
			"status": status,
			"steps":  len(c.flow.Steps),
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("WebSocketFlowChallenge: executed %d steps on %s, status=%s", len(c.flow.Steps), c.flow.URL, status))
	return result, nil
}

// extractWSVariables parses the response as JSON and extracts
// field values into the variables map.
func extractWSVariables(
	response []byte,
	extractTo map[string]string,
	variables map[string]string,
) {
	var respMap map[string]interface{}
	if err := json.Unmarshal(
		response, &respMap,
	); err != nil {
		return
	}

	for field, varName := range extractTo {
		if val, ok := respMap[field]; ok {
			variables[varName] = fmt.Sprintf("%v", val)
		}
	}
}

// evaluateWSStepAssertion evaluates a single step assertion
// against the WebSocket response.
func evaluateWSStepAssertion(
	sa StepAssertion,
	response []byte,
	allResponses [][]byte,
	err error,
) bool {
	if err != nil {
		return false
	}
	switch sa.Type {
	case "response_contains":
		if expected, ok := sa.Value.(string); ok {
			return strings.Contains(
				string(response), expected,
			)
		}
		return false
	case "not_empty":
		return len(response) > 0
	case "message_count":
		if expected, ok := sa.Value.(float64); ok {
			return len(allResponses) >= int(expected)
		}
		if expected, ok := sa.Value.(int); ok {
			return len(allResponses) >= expected
		}
		return false
	default:
		return false
	}
}

// wsConnectActual returns the actual value string for a
// WebSocket connection assertion.
func wsConnectActual(err error) string {
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error())
	}
	return "connected"
}

// wsConnectMessage returns a human-readable message for a
// WebSocket connection assertion.
func wsConnectMessage(err error) string {
	if err != nil {
		return fmt.Sprintf(
			"WebSocket connection failed: %s",
			err.Error(),
		)
	}
	return "WebSocket connection established"
}

// wsStepActual returns the actual value string for a
// WebSocket step assertion.
func wsStepActual(err error) string {
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error())
	}
	return "success"
}

// wsStepMessage returns a human-readable message for a
// WebSocket step assertion.
func wsStepMessage(
	name, action string, err error,
) string {
	if err != nil {
		return fmt.Sprintf(
			"WebSocket step %q (%s) failed: %s",
			name, action, err.Error(),
		)
	}
	return fmt.Sprintf(
		"WebSocket step %q (%s) succeeded",
		name, action,
	)
}
