package runner

import (
	"context"
	"sync"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// livenessMonitor watches a challenge's progress channel and
// cancels the execution context if no progress is reported
// within the stale threshold. This detects stuck challenges
// without penalizing legitimately long-running ones.
//
// A challenge scanning 100k files for hours is fine — as long
// as it keeps calling ReportProgress, the monitor stays quiet.
// But if progress stops for longer than the threshold, the
// challenge is considered stuck and cancelled.
type livenessMonitor struct {
	progress       *challenge.ProgressReporter
	staleThreshold time.Duration
	cancel         context.CancelFunc
	logger         challenge.Logger
	challengeID    challenge.ID
}

// startLivenessMonitor creates and starts a liveness monitor
// goroutine. Returns a stop function that must be called when
// execution completes (to prevent goroutine leaks). The
// monitor runs until stop() is called or the stale threshold
// is exceeded.
//
// If progress is nil or staleThreshold is zero, returns a
// no-op stop function (liveness detection is disabled).
func startLivenessMonitor(
	progress *challenge.ProgressReporter,
	staleThreshold time.Duration,
	cancel context.CancelFunc,
	logger challenge.Logger,
	challengeID challenge.ID,
) (stop func(), stuck <-chan struct{}) {
	if progress == nil || staleThreshold <= 0 {
		return func() { _ = struct{}{} }, nil
	}

	m := &livenessMonitor{
		progress:       progress,
		staleThreshold: staleThreshold,
		cancel:         cancel,
		logger:         logger,
		challengeID:    challengeID,
	}

	stopCh := make(chan struct{})
	stuckCh := make(chan struct{})

	go m.run(stopCh, stuckCh)

	var once sync.Once
	return func() {
		once.Do(func() { close(stopCh) })
	}, stuckCh
}

// run is the main monitor loop. It reads progress updates
// and resets the stale timer on each one. If the timer fires,
// the challenge is considered stuck.
func (m *livenessMonitor) run(
	stopCh <-chan struct{},
	stuckCh chan<- struct{},
) {
	timer := time.NewTimer(m.staleThreshold)
	defer timer.Stop()

	progressCh := m.progress.Channel()

	for {
		select {
		case <-stopCh:
			// Execution completed normally; stop monitoring.
			return

		case _, ok := <-progressCh:
			if !ok {
				// Channel closed; challenge finished.
				return
			}
			// Progress received — reset the stale timer.
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(m.staleThreshold)

		case <-timer.C:
			// No progress within the stale threshold.
			// Challenge is stuck.
			if m.logger != nil {
				m.logger.Error(
					"challenge_stuck",
					"challenge_id", m.challengeID,
					"stale_threshold_seconds",
					m.staleThreshold.Seconds(),
				)
			}
			// Signal stuck first, then cancel context.
			// This ensures the runner sees stuckCh is closed
			// before Execute returns from context cancellation.
			close(stuckCh)
			m.cancel()
			return
		}
	}
}
