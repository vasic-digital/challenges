// Package runner provides the challenge execution engine. It
// supports single, sequential, and parallel execution modes
// with configurable timeouts and lifecycle hooks.
package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/monitor"
	"digital.vasic.challenges/pkg/registry"
)

// Runner defines the interface for challenge execution.
type Runner interface {
	// Run executes a single challenge by ID.
	Run(
		ctx context.Context,
		id challenge.ID,
		config *challenge.Config,
	) (*challenge.Result, error)

	// RunAll executes all challenges in dependency order.
	RunAll(
		ctx context.Context,
		config *challenge.Config,
	) ([]*challenge.Result, error)

	// RunSequence executes the given challenges in order,
	// checking that dependencies have been met.
	RunSequence(
		ctx context.Context,
		ids []challenge.ID,
		config *challenge.Config,
	) ([]*challenge.Result, error)

	// RunParallel executes independent challenges
	// concurrently with the given concurrency limit.
	RunParallel(
		ctx context.Context,
		ids []challenge.ID,
		config *challenge.Config,
		maxConcurrency int,
	) ([]*challenge.Result, error)
}

// ExecuteHook allows testing of error paths in executeChallenge.
// It is called after executeChallenge completes and can override
// the returned error. This is only intended for testing.
type ExecuteHook func(
	c challenge.Challenge,
	result *challenge.Result,
	err error,
) (*challenge.Result, error)

// DefaultRunner is the standard Runner implementation.
type DefaultRunner struct {
	registry       registry.Registry
	logger         challenge.Logger
	eventCollector *monitor.EventCollector
	timeout        time.Duration
	staleThreshold time.Duration
	resultsDir     string
	preHooks       []Hook
	postHooks      []Hook
	executeHook    ExecuteHook // test hook for executeChallenge errors
}

// Hook is a function invoked before or after challenge
// execution. It receives the challenge and its config.
type Hook func(
	ctx context.Context,
	c challenge.Challenge,
	cfg *challenge.Config,
) error

// NewRunner creates a DefaultRunner with the supplied options.
func NewRunner(opts ...RunnerOption) *DefaultRunner {
	r := &DefaultRunner{
		registry: registry.Default,
		timeout:  10 * time.Minute,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run executes a single challenge by ID.
func (r *DefaultRunner) Run(
	ctx context.Context,
	id challenge.ID,
	config *challenge.Config,
) (*challenge.Result, error) {
	c, err := r.registry.Get(id)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get challenge: %w", err,
		)
	}
	return r.executeChallenge(ctx, c, config)
}

// RunAll executes all challenges in dependency order. If a
// challenge passes, its results directory is propagated to
// downstream dependents.
func (r *DefaultRunner) RunAll(
	ctx context.Context,
	config *challenge.Config,
) ([]*challenge.Result, error) {
	ordered, err := r.registry.GetDependencyOrder()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get dependency order: %w", err,
		)
	}

	var results []*challenge.Result
	depResults := make(map[challenge.ID]string)

	for _, c := range ordered {
		cfg := *config
		cfg.ChallengeID = c.ID()
		cfg.Dependencies = depResults

		result, execErr := r.executeChallenge(ctx, c, &cfg)
		if execErr != nil {
			return results, fmt.Errorf(
				"challenge %s failed: %w",
				c.ID(), execErr,
			)
		}

		results = append(results, result)

		if result.Status == challenge.StatusPassed {
			depResults[c.ID()] = cfg.ResultsDir
		}
	}

	return results, nil
}

// RunSequence executes challenges in dependency order (Kahn topological
// sort), verifying that each challenge's dependencies have already been
// executed and passed within this sequence.
func (r *DefaultRunner) RunSequence(
	ctx context.Context,
	ids []challenge.ID,
	config *challenge.Config,
) ([]*challenge.Result, error) {
	// Topological sort (Kahn's algorithm) so callers are not required
	// to pre-sort challenge IDs manually.
	sorted, err := r.topoSort(ids)
	if err != nil {
		return nil, fmt.Errorf("run sequence: %w", err)
	}

	var results []*challenge.Result
	depResults := make(map[challenge.ID]string)

	for _, id := range sorted {
		c, err := r.registry.Get(id)
		if err != nil {
			return results, fmt.Errorf(
				"failed to get challenge %s: %w", id, err,
			)
		}

		for _, dep := range c.Dependencies() {
			if _, exists := depResults[dep]; !exists {
				return results, fmt.Errorf(
					"challenge %s has unmet dependency: %s",
					id, dep,
				)
			}
		}

		cfg := *config
		cfg.ChallengeID = id
		cfg.Dependencies = depResults

		result, execErr := r.executeChallenge(ctx, c, &cfg)
		if execErr != nil {
			return results, fmt.Errorf(
				"challenge %s failed: %w", id, execErr,
			)
		}

		results = append(results, result)

		if result.Status == challenge.StatusPassed {
			depResults[id] = cfg.ResultsDir
		}
	}

	return results, nil
}

// topoSort performs Kahn's topological sort on the given challenge IDs
// using their declared dependencies. All IDs and their transitive deps
// must be present in the registry. Returns an error on cyclic deps.
func (r *DefaultRunner) topoSort(
	ids []challenge.ID,
) ([]challenge.ID, error) {
	idSet := make(map[challenge.ID]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	inDegree := make(map[challenge.ID]int, len(ids))
	adj := make(map[challenge.ID][]challenge.ID, len(ids))

	for _, id := range ids {
		c, err := r.registry.Get(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get challenge %s: %w", id, err)
		}
		inDegree[id] = 0
		for _, dep := range c.Dependencies() {
			if _, ok := idSet[dep]; !ok {
				continue // dependency outside the sequence is ignored
			}
			inDegree[id]++
			adj[dep] = append(adj[dep], id)
		}
	}

	queue := make([]challenge.ID, 0, len(ids))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []challenge.ID
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, id)
		for _, next := range adj[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(sorted) != len(ids) {
		return nil, fmt.Errorf("cyclic dependency detected in challenge sequence")
	}

	return sorted, nil
}

// RunParallel executes the given challenges concurrently using
// at most maxConcurrency goroutines. It delegates to the
// parallel runner implementation.
func (r *DefaultRunner) RunParallel(
	ctx context.Context,
	ids []challenge.ID,
	config *challenge.Config,
	maxConcurrency int,
) ([]*challenge.Result, error) {
	return runParallel(ctx, r, ids, config, maxConcurrency)
}

// executeChallenge runs a single challenge through its full
// lifecycle: setup dir -> pre-hooks -> configure -> validate ->
// execute with timeout -> evaluate assertions -> post-hooks ->
// cleanup.
func (r *DefaultRunner) executeChallenge(
	ctx context.Context,
	c challenge.Challenge,
	config *challenge.Config,
) (*challenge.Result, error) {
	result := &challenge.Result{
		ChallengeID:   c.ID(),
		ChallengeName: c.Name(),
		Status:        challenge.StatusRunning,
		StartTime:     time.Now(),
		Metrics:       make(map[string]challenge.MetricValue),
		Outputs:       make(map[string]string),
	}

	// Setup results directory.
	if err := r.setupResultsDir(config); err != nil {
		result.Status = challenge.StatusError
		result.Error = fmt.Sprintf(
			"failed to setup results directory: %v", err,
		)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	result.Logs = challenge.LogPaths{
		ChallengeLog: filepath.Join(
			config.LogsDir, "challenge.log",
		),
		OutputLog: filepath.Join(
			config.LogsDir, "output.log",
		),
	}

	r.logEvent("challenge_started", map[string]any{
		"challenge_id":   c.ID(),
		"challenge_name": c.Name(),
	})
	r.emitEvent(monitor.EventStarted, c.ID(), c.Name())

	// Pre-hooks.
	for _, hook := range r.preHooks {
		if err := hook(ctx, c, config); err != nil {
			result.Status = challenge.StatusError
			result.Error = fmt.Sprintf(
				"pre-hook failed: %v", err,
			)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(
				result.StartTime,
			)
			r.emitEvent(monitor.EventFailed, c.ID(), c.Name(), map[string]interface{}{
				"error": result.Error,
			})
			return result, nil
		}
	}

	// Configure.
	if err := c.Configure(config); err != nil {
		result.Status = challenge.StatusError
		result.Error = fmt.Sprintf(
			"configuration failed: %v", err,
		)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_error", map[string]any{
			"challenge_id": c.ID(),
			"error":        result.Error,
		})
		r.emitEvent(monitor.EventFailed, c.ID(), c.Name(), map[string]interface{}{
			"error": result.Error,
		})
		return result, nil
	}

	r.emitEvent(monitor.EventConfigured, c.ID(), c.Name())

	// Validate.
	if err := c.Validate(ctx); err != nil {
		result.Status = challenge.StatusSkipped
		result.Error = fmt.Sprintf(
			"validation failed: %v", err,
		)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_skipped", map[string]any{
			"challenge_id": c.ID(),
			"reason":       result.Error,
		})
		r.emitEvent(monitor.EventSkipped, c.ID(), c.Name(), map[string]interface{}{
			"reason": result.Error,
		})
		return result, nil
	}

	r.emitEvent(monitor.EventValidated, c.ID(), c.Name())

	// Setup progress-based liveness detection. If the
	// challenge supports progress reporting, attach a
	// ProgressReporter so the liveness monitor can track
	// forward progress. This allows long-running challenges
	// (hours) while detecting stuck ones (no progress).
	var progress *challenge.ProgressReporter
	type progressAware interface {
		SetProgressReporter(*challenge.ProgressReporter)
	}
	if pa, ok := c.(progressAware); ok {
		progress = challenge.NewProgressReporter()
		pa.SetProgressReporter(progress)
		defer progress.Close()
	}

	// Determine stale threshold: per-challenge config
	// overrides the runner default.
	staleThreshold := config.StaleThreshold
	if staleThreshold == 0 {
		staleThreshold = r.staleThreshold
	}

	// Execute with timeout. The timeout is a hard upper
	// bound; the liveness monitor provides a softer
	// progress-based check within that window.
	timeout := config.Timeout
	if timeout == 0 {
		timeout = r.timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start liveness monitor before Execute. It watches
	// the progress channel and cancels execCtx if no
	// progress is reported within the stale threshold.
	stopLiveness, stuckCh := startLivenessMonitor(
		progress, staleThreshold, cancel,
		r.logger, c.ID(),
	)
	defer stopLiveness()

	r.emitEvent(monitor.EventExecuting, c.ID(), c.Name())

	execResult, execErr := c.Execute(execCtx)

	// Stop liveness monitor immediately after Execute
	// returns to prevent false stuck detection during
	// post-processing.
	stopLiveness()

	r.emitEvent(monitor.EventExecutingCompleted, c.ID(), c.Name(), map[string]interface{}{
		"duration": time.Since(result.StartTime),
	})

	// Check if the challenge was killed due to no
	// progress (stuck) vs hard timeout vs normal error.
	wasStuck := false
	if stuckCh != nil {
		select {
		case <-stuckCh:
			wasStuck = true
		default:
		}
	}

	// Handle stuck challenge (no progress within stale
	// threshold). This takes priority over timeout since
	// the liveness monitor cancelled the context.
	if wasStuck {
		result.Status = challenge.StatusStuck
		result.Error = fmt.Sprintf(
			"challenge stuck: no progress reported "+
				"within %v", staleThreshold,
		)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(
			result.StartTime,
		)
		r.logEvent("challenge_stuck", map[string]any{
			"challenge_id":            c.ID(),
			"stale_threshold_seconds": staleThreshold.Seconds(),
		})
		r.emitEvent(monitor.EventStuck, c.ID(), c.Name(), map[string]interface{}{
			"stale_threshold_seconds": staleThreshold.Seconds(),
			"error":                   result.Error,
		})
		_ = c.Cleanup(ctx)
		return result, nil
	}

	// Handle timeout.
	if execCtx.Err() == context.DeadlineExceeded {
		result.Status = challenge.StatusTimedOut
		result.Error = "challenge execution timed out"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_timeout", map[string]any{
			"challenge_id":    c.ID(),
			"timeout_seconds": timeout.Seconds(),
		})
		r.emitEvent(monitor.EventTimedOut, c.ID(), c.Name(), map[string]interface{}{
			"timeout_seconds": timeout.Seconds(),
			"error":           result.Error,
		})
		_ = c.Cleanup(ctx)
		return result, nil
	}

	// Handle execution error.
	if execErr != nil {
		result.Status = challenge.StatusError
		result.Error = fmt.Sprintf(
			"execution failed: %v", execErr,
		)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_error", map[string]any{
			"challenge_id": c.ID(),
			"error":        result.Error,
		})
		r.emitEvent(monitor.EventFailed, c.ID(), c.Name(), map[string]interface{}{
			"error": result.Error,
		})
		_ = c.Cleanup(ctx)
		return result, nil
	}

	// Merge execution result.
	if execResult != nil {
		result.Assertions = execResult.Assertions
		result.RecordedActions = execResult.RecordedActions
		result.Metrics = execResult.Metrics
		result.Outputs = execResult.Outputs
		// Preserve execution status if it indicates failure
		if execResult.Status == challenge.StatusFailed ||
			execResult.Status == challenge.StatusTimedOut ||
			execResult.Status == challenge.StatusError {
			result.Status = execResult.Status
			result.Error = execResult.Error
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			r.logEvent("challenge_failed", map[string]any{
				"challenge_id": c.ID(),
				"status":       result.Status,
				"error":        result.Error,
			})
			_ = c.Cleanup(ctx)
			return result, nil
		}
	}

	// Determine final status from assertions.
	result.Status = challenge.StatusPassed
	for _, a := range result.Assertions {
		if !a.Passed {
			result.Status = challenge.StatusFailed
			break
		}
	}

		// Anti-bluff validation is mandatory per Constitution §1, §6.3,
		// §11.5.7. A Challenge result claiming Status=Passed MUST carry
		// positive evidence (RecordedActions non-empty + at least one
		// passing assertion). This gate is never disabled; the env-var
		// CHALLENGE_ANTIBLUFF_STRICT has been removed as part of the
		// v2.0.0 constitutional amendment (2026-05-01).
	if result.Status == challenge.StatusPassed {
		if abErr := challenge.ValidateAntiBluff(result); abErr != nil {
			result.Status = challenge.StatusFailed
			if result.Error == "" {
				result.Error = abErr.Error()
			} else {
				result.Error = result.Error + "; " + abErr.Error()
			}
		}
	}

	// Calculate assertion stats for event
	totalAssertions := len(result.Assertions)
	passedAssertions := 0
	for _, a := range result.Assertions {
		if a.Passed {
			passedAssertions++
		}
	}
	r.emitEvent(monitor.EventAssertionsEvaluated, c.ID(), c.Name(), map[string]interface{}{
		"passed": passedAssertions,
		"total":  totalAssertions,
	})

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Post-hooks.
	for _, hook := range r.postHooks {
		if err := hook(ctx, c, config); err != nil {
			r.logEvent("post_hook_warning", map[string]any{
				"challenge_id": c.ID(),
				"warning":      err.Error(),
			})
		}
	}

	r.logEvent("challenge_completed", map[string]any{
		"challenge_id":     c.ID(),
		"status":           result.Status,
		"duration_seconds": result.Duration.Seconds(),
	})
	r.emitEvent(monitor.EventCompleted, c.ID(), c.Name(), map[string]interface{}{
		"status":   string(result.Status),
		"duration": result.Duration,
	})
	r.emitEvent(monitor.EventCleanupStarted, c.ID(), c.Name())

	// Cleanup.
	if err := c.Cleanup(ctx); err != nil {
		r.logEvent("cleanup_warning", map[string]any{
			"challenge_id": c.ID(),
			"warning":      err.Error(),
		})
	}
	r.emitEvent(monitor.EventCleanupCompleted, c.ID(), c.Name())

	// Apply test hook if set.
	if r.executeHook != nil {
		return r.executeHook(c, result, nil)
	}

	return result, nil
}

// setupResultsDir creates the results directory structure.
func (r *DefaultRunner) setupResultsDir(
	config *challenge.Config,
) error {
	if config.ResultsDir == "" {
		now := time.Now()
		baseDir := r.resultsDir
		if baseDir == "" {
			baseDir = "results"
		}

		config.ResultsDir = filepath.Join(
			baseDir,
			string(config.ChallengeID),
			now.Format("2006"),
			now.Format("01"),
			now.Format("02"),
			now.Format("20060102_150405"),
		)
	}

	config.LogsDir = filepath.Join(
		config.ResultsDir, "logs",
	)

	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return err
	}

	resultsDir := filepath.Join(
		config.ResultsDir, "results",
	)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return err
	}

	configDir := filepath.Join(
		config.ResultsDir, "config",
	)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	return nil
}

// eventEmittingProgressReporter wraps a ProgressReporter to emit
// EventProgress events for each progress update.
type eventEmittingProgressReporter struct {
	*challenge.ProgressReporter
	eventCollector *monitor.EventCollector
	challengeID    challenge.ID
	name           string
}

// ReportProgress emits a progress event before delegating to the
// embedded ProgressReporter.
func (p *eventEmittingProgressReporter) ReportProgress(
	msg string,
	data map[string]any,
) {
	if p.eventCollector != nil {
		p.eventCollector.EmitProgress(p.challengeID, p.name, msg, data)
	}
	p.ProgressReporter.ReportProgress(msg, data)
}

// newEventEmittingProgressReporter creates a wrapped progress reporter
// that emits events for each progress update.
func newEventEmittingProgressReporter(
	base *challenge.ProgressReporter,
	eventCollector *monitor.EventCollector,
	challengeID challenge.ID,
	name string,
) *eventEmittingProgressReporter {
	return &eventEmittingProgressReporter{
		ProgressReporter: base,
		eventCollector:   eventCollector,
		challengeID:      challengeID,
		name:             name,
	}
}

// logEvent emits a structured log entry if a logger is
// configured.
func (r *DefaultRunner) logEvent(
	event string,
	data map[string]any,
) {
	if r.logger == nil {
		return
	}

	parts := make([]any, 0, len(data)*2)
	for k, v := range data {
		parts = append(parts, k, v)
	}
	r.logger.Info(event, parts...)
}

// emitEvent emits a challenge lifecycle event to the event collector
// if configured.
func (r *DefaultRunner) emitEvent(
	eventType monitor.EventType,
	challengeID challenge.ID,
	name string,
	additionalFields ...map[string]interface{},
) {
	if r.eventCollector == nil {
		return
	}

	event := monitor.ChallengeEvent{
		Type:        eventType,
		ChallengeID: challengeID,
		Name:        name,
		Timestamp:   time.Now(),
	}

	if len(additionalFields) > 0 {
		fields := additionalFields[0]
		if status, ok := fields["status"].(string); ok {
			event.Status = status
		}
		if message, ok := fields["message"].(string); ok {
			event.Message = message
		}
		if duration, ok := fields["duration"].(time.Duration); ok {
			event.Duration = duration
		}
		if metrics, ok := fields["metrics"].(map[string]interface{}); ok {
			event.Metrics = metrics
		}
		if progressData, ok := fields["progress_data"].(map[string]interface{}); ok {
			event.ProgressData = progressData
		}
		if errMsg, ok := fields["error"].(string); ok {
			event.Error = errMsg
		}
		if stage, ok := fields["stage"].(string); ok {
			event.Stage = stage
		}
	}

	r.eventCollector.Emit(event)
}
