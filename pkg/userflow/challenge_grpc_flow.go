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
var _ challenge.Challenge = (*GRPCFlowChallenge)(nil)

// GRPCFlow defines a sequence of gRPC steps to execute as a
// user flow test.
type GRPCFlow struct {
	// ServerAddr is the gRPC server address (host:port).
	ServerAddr string `json:"server_addr"`

	// Options holds connection-level configuration.
	Options GRPCFlowOptions `json:"options"`

	// Steps is the ordered sequence of gRPC steps.
	Steps []GRPCStep `json:"steps"`
}

// GRPCFlowOptions configures the gRPC connection for all
// steps in a flow.
type GRPCFlowOptions struct {
	// Insecure disables TLS certificate verification.
	Insecure bool `json:"insecure,omitempty"`

	// Headers are metadata headers sent with every request.
	Headers map[string]string `json:"headers,omitempty"`
}

// GRPCStep defines a single step in a gRPC flow.
type GRPCStep struct {
	// Name identifies this step.
	Name string `json:"name"`

	// Method is the full gRPC method path
	// (e.g., "package.Service/Method").
	Method string `json:"method"`

	// Request is the JSON request body.
	Request string `json:"request,omitempty"`

	// Stream indicates this is a server-streaming call.
	Stream bool `json:"stream,omitempty"`

	// ExpectedFields maps response JSON field names to their
	// expected values. Nil values mean "field must exist".
	ExpectedFields map[string]interface{} `json:"expected_fields,omitempty"`

	// Assertions define checks to run on the response.
	Assertions []StepAssertion `json:"assertions"`

	// ExtractTo maps response JSON field names to variable
	// names. Extracted values can be referenced in subsequent
	// steps via {{var_name}} placeholders.
	ExtractTo map[string]string `json:"extract_to,omitempty"`
}

// GRPCFlowChallenge executes a multi-step gRPC flow using a
// GRPCAdapter. It iterates through steps, invoking methods
// with variable substitution and assertion evaluation.
type GRPCFlowChallenge struct {
	challenge.BaseChallenge
	adapter GRPCAdapter
	flow    GRPCFlow
}

// NewGRPCFlowChallenge creates a challenge that executes the
// given GRPCFlow using the provided adapter.
func NewGRPCFlowChallenge(
	id, name, description string,
	deps []challenge.ID,
	adapter GRPCAdapter,
	flow GRPCFlow,
) *GRPCFlowChallenge {
	return &GRPCFlowChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			name,
			description,
			"grpc",
			deps,
		),
		adapter: adapter,
		flow:    flow,
	}
}

// Execute runs the full gRPC flow: verifies availability,
// then executes each step in order with variable substitution,
// field validation, and assertion checking.
func (c *GRPCFlowChallenge) Execute(
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
		result.RecordAction(fmt.Sprintf("GRPCFlowChallenge: platform not available, skipped (%d steps, server=%s)", len(c.flow.Steps), c.flow.ServerAddr))
		return result, nil
	}

	var assertions []challenge.AssertionResult
	metrics := make(map[string]challenge.MetricValue)
	outputs := make(map[string]string)
	variables := make(map[string]string)
	allPassed := true

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

		// Substitute variables in method and request.
		method := substituteVars(step.Method, variables)
		request := substituteVars(step.Request, variables)

		// Execute the gRPC call.
		var respStr string
		var respList []string
		var stepErr error

		if step.Stream {
			respList, stepErr = c.adapter.InvokeStream(
				ctx, method, request,
			)
			if stepErr == nil && len(respList) > 0 {
				respStr = respList[0]
			}
		} else {
			respStr, stepErr = c.adapter.Invoke(
				ctx, method, request,
			)
		}

		// Check for invocation errors.
		invokePassed := stepErr == nil
		if !invokePassed {
			allPassed = false
		}
		assertions = append(
			assertions, challenge.AssertionResult{
				Type:     "grpc_invoke",
				Target:   step.Name,
				Expected: "success",
				Actual:   grpcInvokeActual(stepErr),
				Passed:   invokePassed,
				Message: grpcInvokeMessage(
					step.Name, stepErr,
				),
			},
		)

		// Validate expected fields.
		if stepErr == nil && len(step.ExpectedFields) > 0 {
			fieldAssertions := validateGRPCFields(
				step.Name, respStr, step.ExpectedFields,
			)
			for _, fa := range fieldAssertions {
				if !fa.Passed {
					allPassed = false
				}
			}
			assertions = append(assertions, fieldAssertions...)
		}

		// Validate streaming responses if applicable.
		if step.Stream && stepErr == nil {
			streamPassed := len(respList) > 0
			if !streamPassed {
				allPassed = false
			}
			assertions = append(
				assertions, challenge.AssertionResult{
					Type:     "grpc_stream",
					Target:   step.Name,
					Expected: "at least 1 response",
					Actual: fmt.Sprintf(
						"%d responses", len(respList),
					),
					Passed: streamPassed,
					Message: grpcStreamMessage(
						step.Name, len(respList),
					),
				},
			)
		}

		// Extract variables from response.
		if stepErr == nil && len(step.ExtractTo) > 0 {
			extractGRPCVariables(
				respStr, step.ExtractTo, variables,
			)
		}

		// Evaluate step assertions.
		for _, sa := range step.Assertions {
			saPassed := evaluateGRPCStepAssertion(
				sa, respStr, respList, stepErr,
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
		if respStr != "" {
			outputs[step.Name] = respStr
		}
		if step.Stream && len(respList) > 0 {
			data, jsonErr := json.Marshal(respList)
			if jsonErr == nil {
				outputs[step.Name+"_stream"] = string(data)
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

	c.ReportProgress("gRPC flow complete", map[string]any{
		"status": status,
		"steps":  len(c.flow.Steps),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	result.RecordAction(fmt.Sprintf("GRPCFlowChallenge: executed %d steps on %s, status=%s", len(c.flow.Steps), c.flow.ServerAddr, status))
	return result, nil
}

// validateGRPCFields parses the JSON response and checks
// that each expected field exists and optionally matches the
// expected value.
func validateGRPCFields(
	stepName, response string,
	expected map[string]interface{},
) []challenge.AssertionResult {
	var results []challenge.AssertionResult

	var respMap map[string]interface{}
	if err := json.Unmarshal(
		[]byte(response), &respMap,
	); err != nil {
		results = append(
			results, challenge.AssertionResult{
				Type:   "grpc_field",
				Target: stepName,
				Passed: false,
				Message: fmt.Sprintf(
					"cannot parse response JSON: %s",
					err.Error(),
				),
			},
		)
		return results
	}

	for field, expectedVal := range expected {
		actualVal, exists := respMap[field]
		if !exists {
			results = append(
				results, challenge.AssertionResult{
					Type:     "grpc_field",
					Target:   fmt.Sprintf("%s.%s", stepName, field),
					Expected: fmt.Sprintf("%v", expectedVal),
					Actual:   "field missing",
					Passed:   false,
					Message: fmt.Sprintf(
						"field %q not found in response",
						field,
					),
				},
			)
			continue
		}

		// Nil expected means "field must exist" only.
		if expectedVal == nil {
			results = append(
				results, challenge.AssertionResult{
					Type: "grpc_field",
					Target: fmt.Sprintf(
						"%s.%s", stepName, field,
					),
					Expected: "exists",
					Actual:   fmt.Sprintf("%v", actualVal),
					Passed:   true,
					Message: fmt.Sprintf(
						"field %q exists", field,
					),
				},
			)
			continue
		}

		// Compare as strings for simplicity.
		expStr := fmt.Sprintf("%v", expectedVal)
		actStr := fmt.Sprintf("%v", actualVal)
		passed := expStr == actStr

		results = append(
			results, challenge.AssertionResult{
				Type:     "grpc_field",
				Target:   fmt.Sprintf("%s.%s", stepName, field),
				Expected: expStr,
				Actual:   actStr,
				Passed:   passed,
				Message: grpcFieldMessage(
					field, passed, expStr, actStr,
				),
			},
		)
	}

	return results
}

// extractGRPCVariables parses the JSON response and extracts
// field values into the variables map.
func extractGRPCVariables(
	response string,
	extractTo map[string]string,
	variables map[string]string,
) {
	var respMap map[string]interface{}
	if err := json.Unmarshal(
		[]byte(response), &respMap,
	); err != nil {
		return
	}

	for field, varName := range extractTo {
		if val, ok := respMap[field]; ok {
			variables[varName] = fmt.Sprintf("%v", val)
		}
	}
}

// evaluateGRPCStepAssertion evaluates a single step assertion
// against the gRPC response.
func evaluateGRPCStepAssertion(
	sa StepAssertion,
	response string,
	streamResponses []string,
	err error,
) bool {
	if err != nil {
		return false
	}
	switch sa.Type {
	case "response_contains":
		if expected, ok := sa.Value.(string); ok {
			return strings.Contains(response, expected)
		}
		return false
	case "not_empty":
		return response != ""
	case "stream_count":
		if expected, ok := sa.Value.(float64); ok {
			return len(streamResponses) >= int(expected)
		}
		if expected, ok := sa.Value.(int); ok {
			return len(streamResponses) >= expected
		}
		return false
	default:
		return false
	}
}

// grpcInvokeActual returns the actual value string for a
// gRPC invocation assertion.
func grpcInvokeActual(err error) string {
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error())
	}
	return "success"
}

// grpcInvokeMessage returns a human-readable message for a
// gRPC invocation assertion.
func grpcInvokeMessage(stepName string, err error) string {
	if err != nil {
		return fmt.Sprintf(
			"gRPC step %q failed: %s",
			stepName, err.Error(),
		)
	}
	return fmt.Sprintf(
		"gRPC step %q succeeded", stepName,
	)
}

// grpcStreamMessage returns a human-readable message for a
// streaming response count assertion.
func grpcStreamMessage(stepName string, count int) string {
	if count == 0 {
		return fmt.Sprintf(
			"gRPC stream %q returned no responses",
			stepName,
		)
	}
	return fmt.Sprintf(
		"gRPC stream %q returned %d response(s)",
		stepName, count,
	)
}

// grpcFieldMessage returns a human-readable message for a
// gRPC field validation assertion.
func grpcFieldMessage(
	field string, passed bool, expected, actual string,
) string {
	if passed {
		return fmt.Sprintf(
			"field %q matches expected value %q",
			field, expected,
		)
	}
	return fmt.Sprintf(
		"field %q expected %q but got %q",
		field, expected, actual,
	)
}
