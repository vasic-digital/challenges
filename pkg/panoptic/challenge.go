package panoptic

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"digital.vasic.challenges/pkg/assertion"
	"digital.vasic.challenges/pkg/challenge"
)

// EngineAdapter wraps assertion.DefaultEngine to implement
// challenge.AssertionEngine, bridging the two structurally
// equivalent type systems.
type EngineAdapter struct {
	engine *assertion.DefaultEngine
}

// NewEngineAdapter creates an adapter from an assertion engine.
func NewEngineAdapter(
	engine *assertion.DefaultEngine,
) *EngineAdapter {
	return &EngineAdapter{engine: engine}
}

// Evaluate delegates to the assertion engine, converting types.
func (a *EngineAdapter) Evaluate(
	def challenge.AssertionDef,
	value any,
) challenge.AssertionResult {
	result := a.engine.Evaluate(assertion.Definition{
		Type:    def.Type,
		Target:  def.Target,
		Value:   def.Value,
		Values:  def.Values,
		Message: def.Message,
	}, value)

	return challenge.AssertionResult{
		Type:     result.Type,
		Target:   result.Target,
		Expected: result.Expected,
		Actual:   result.Actual,
		Passed:   result.Passed,
		Message:  result.Message,
	}
}

// EvaluateAll delegates to the assertion engine, converting
// types for each assertion.
func (a *EngineAdapter) EvaluateAll(
	defs []challenge.AssertionDef,
	values map[string]any,
) []challenge.AssertionResult {
	assertionDefs := make([]assertion.Definition, len(defs))
	for i, d := range defs {
		assertionDefs[i] = assertion.Definition{
			Type:    d.Type,
			Target:  d.Target,
			Value:   d.Value,
			Values:  d.Values,
			Message: d.Message,
		}
	}

	results := a.engine.EvaluateAll(assertionDefs, values)

	challengeResults := make(
		[]challenge.AssertionResult, len(results),
	)
	for i, r := range results {
		challengeResults[i] = challenge.AssertionResult{
			Type:     r.Type,
			Target:   r.Target,
			Expected: r.Expected,
			Actual:   r.Actual,
			Passed:   r.Passed,
			Message:  r.Message,
		}
	}

	return challengeResults
}

// PanopticChallenge executes Panoptic UI tests as a Challenge.
// It wraps a PanopticAdapter and maps results to the Challenge
// assertion and metric system.
type PanopticChallenge struct {
	challenge.BaseChallenge

	adapter       PanopticAdapter
	configPath    string
	configBuilder *ConfigBuilder
	runOpts       []RunOption
	assertionDefs []challenge.AssertionDef
}

// NewPanopticChallenge creates a PanopticChallenge with the
// given identity and adapter. Use ChallengeOption functions
// to configure the config path, builder, and run options.
func NewPanopticChallenge(
	id challenge.ID,
	name, description, category string,
	deps []challenge.ID,
	adapter PanopticAdapter,
	assertions []challenge.AssertionDef,
	opts ...ChallengeOption,
) *PanopticChallenge {
	c := &PanopticChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			id, name, description, category, deps,
		),
		adapter:       adapter,
		assertionDefs: assertions,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Validate checks that the adapter is available and a config
// source is set.
func (c *PanopticChallenge) Validate(
	ctx context.Context,
) error {
	if err := c.BaseChallenge.Validate(ctx); err != nil {
		return err
	}

	if c.adapter == nil {
		return fmt.Errorf(
			"challenge %s: panoptic adapter is nil", c.ID(),
		)
	}

	if !c.adapter.Available(ctx) {
		return fmt.Errorf(
			"challenge %s: panoptic binary not available",
			c.ID(),
		)
	}

	if c.configPath == "" && c.configBuilder == nil {
		return fmt.Errorf(
			"challenge %s: no config path or builder set",
			c.ID(),
		)
	}

	return nil
}

// Execute runs the Panoptic config, parses results, evaluates
// assertions, and returns a Challenge Result.
func (c *PanopticChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()

	// If a config builder is set, write the config to a
	// temporary file in the results directory.
	configPath := c.configPath
	if c.configBuilder != nil {
		genPath := filepath.Join(
			c.ResultsDir(), "panoptic_config.yaml",
		)
		if err := c.configBuilder.WriteYAML(genPath); err != nil {
			result := c.CreateResult(
				challenge.StatusError, start, nil, nil, nil,
				fmt.Sprintf(
					"failed to write generated config: %v",
					err,
				),
			)
			result.RecordAction(fmt.Sprintf("PanopticChallenge: failed to write generated config to %s", genPath))
			return result, nil
		}
		configPath = genPath
	}

	// Run Panoptic.
	result, err := c.adapter.Run(ctx, configPath, c.runOpts...)
	if err != nil {
		result := c.CreateResult(
			challenge.StatusError, start, nil, nil, nil,
			fmt.Sprintf("panoptic execution error: %v", err),
		)
		result.RecordAction(fmt.Sprintf("PanopticChallenge: panoptic execution error (config=%s)", configPath))
		return result, nil
	}

	// Parse results into assertion values and metrics.
	values := ParseResultToAssertionValues(result)
	metrics := ParseResultToMetrics(result)

	// Build outputs map.
	outputs := map[string]string{
		"exit_code": fmt.Sprintf("%d", result.ExitCode),
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
	}
	if result.ReportHTML != "" {
		outputs["report_html"] = result.ReportHTML
	}
	if result.ReportJSON != "" {
		outputs["report_json"] = result.ReportJSON
	}
	if result.AIErrorReport != "" {
		outputs["ai_error_report"] = result.AIErrorReport
	}
	if result.AIGeneratedTests != "" {
		outputs["ai_generated_tests"] = result.AIGeneratedTests
	}
	if result.VisionReport != "" {
		outputs["vision_report"] = result.VisionReport
	}

	// Evaluate assertions.
	assertions := c.EvaluateAssertions(
		c.assertionDefs, values,
	)

	// Determine status.
	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	// If no assertions but exit code non-zero, fail.
	if len(assertions) == 0 && result.ExitCode != 0 {
		status = challenge.StatusFailed
	}

	challengeResult := c.CreateResult(
		status, start, assertions, metrics, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("PanopticChallenge: executed with config=%s, exit_code=%d, status=%s", configPath, result.ExitCode, status))
	return challengeResult, nil
}
