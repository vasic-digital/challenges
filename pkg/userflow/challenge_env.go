package userflow

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// EnvironmentSetupChallenge is the root dependency for user
// flow testing. It executes a user-provided setup function
// that typically starts test containers, seeds data, or
// configures the environment.
type EnvironmentSetupChallenge struct {
	challenge.BaseChallenge
	setupFunc func(ctx context.Context) error
	timeout   time.Duration
}

// NewEnvironmentSetupChallenge creates a new environment setup
// challenge. The setupFunc is called during Execute to prepare
// the test environment. The timeout limits how long the setup
// may take; zero means no timeout.
func NewEnvironmentSetupChallenge(
	id string,
	setupFunc func(ctx context.Context) error,
	timeout time.Duration,
) *EnvironmentSetupChallenge {
	return &EnvironmentSetupChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			"Environment Setup",
			"Starts test containers and prepares the "+
				"environment for user flow testing",
			"environment",
			nil,
		),
		setupFunc: setupFunc,
		timeout:   timeout,
	}
}

// Execute runs the setup function and reports success or
// failure as challenge assertions.
func (c *EnvironmentSetupChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()
	c.ReportProgress("starting environment setup", nil)

	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	var errMsg string
	setupErr := c.setupFunc(ctx)

	assertions := []challenge.AssertionResult{
		{
			Type:     "environment_setup",
			Target:   "setup_succeeds",
			Expected: "true",
			Actual:   fmt.Sprintf("%t", setupErr == nil),
			Passed:   setupErr == nil,
			Message:  envSetupMessage(setupErr),
		},
	}

	duration := time.Since(start)
	metrics := map[string]challenge.MetricValue{
		"setup_duration": {
			Name:  "setup_duration",
			Value: duration.Seconds(),
			Unit:  "s",
		},
	}

	status := challenge.StatusPassed
	if setupErr != nil {
		status = challenge.StatusFailed
		errMsg = setupErr.Error()
	}

	c.ReportProgress("environment setup complete", map[string]any{
		"status":   status,
		"duration": duration.String(),
	})

	result := c.CreateResult(
		status, start, assertions, metrics, nil, errMsg,
	)
	result.RecordAction(fmt.Sprintf("EnvironmentSetupChallenge: setup completed, status=%s, duration=%s", status, duration.String()))
	return result, nil
}

// envSetupMessage returns a human-readable message for the
// setup assertion.
func envSetupMessage(err error) string {
	if err == nil {
		return "environment setup completed successfully"
	}
	return fmt.Sprintf(
		"environment setup failed: %s", err.Error(),
	)
}

// EnvironmentTeardownChallenge is the final challenge in a
// user flow pipeline. It executes a user-provided teardown
// function that stops containers and cleans up resources.
// It has no dependents.
type EnvironmentTeardownChallenge struct {
	challenge.BaseChallenge
	teardownFunc func(ctx context.Context) error
}

// NewEnvironmentTeardownChallenge creates a new environment
// teardown challenge. The teardownFunc is called during Execute
// to clean up the test environment.
func NewEnvironmentTeardownChallenge(
	id string,
	teardownFunc func(ctx context.Context) error,
) *EnvironmentTeardownChallenge {
	return &EnvironmentTeardownChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			challenge.ID(id),
			"Environment Teardown",
			"Stops test containers and cleans up the "+
				"environment after user flow testing",
			"environment",
			nil,
		),
		teardownFunc: teardownFunc,
	}
}

// Execute runs the teardown function and reports success
// or failure.
func (c *EnvironmentTeardownChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {
	start := time.Now()
	c.ReportProgress("starting environment teardown", nil)

	var errMsg string
	teardownErr := c.teardownFunc(ctx)

	assertions := []challenge.AssertionResult{
		{
			Type:     "environment_teardown",
			Target:   "teardown_succeeds",
			Expected: "true",
			Actual: fmt.Sprintf(
				"%t", teardownErr == nil,
			),
			Passed:  teardownErr == nil,
			Message: envTeardownMessage(teardownErr),
		},
	}

	duration := time.Since(start)
	metrics := map[string]challenge.MetricValue{
		"teardown_duration": {
			Name:  "teardown_duration",
			Value: duration.Seconds(),
			Unit:  "s",
		},
	}

	status := challenge.StatusPassed
	if teardownErr != nil {
		status = challenge.StatusFailed
		errMsg = teardownErr.Error()
	}

	c.ReportProgress(
		"environment teardown complete",
		map[string]any{
			"status":   status,
			"duration": duration.String(),
		},
	)

	result := c.CreateResult(
		status, start, assertions, metrics, nil, errMsg,
	)
	result.RecordAction(fmt.Sprintf("EnvironmentTeardownChallenge: teardown completed, status=%s, duration=%s", status, duration.String()))
	return result, nil
}

// envTeardownMessage returns a human-readable message for
// the teardown assertion.
func envTeardownMessage(err error) string {
	if err == nil {
		return "environment teardown completed successfully"
	}
	return fmt.Sprintf(
		"environment teardown failed: %s", err.Error(),
	)
}
