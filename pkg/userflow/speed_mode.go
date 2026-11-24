// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package userflow

import (
	"context"
	"math/rand"
	"time"
)

// SpeedMode defines human interaction speed presets for UI
// automation. Three modes simulate different user behaviors:
// slow (elderly/careful), normal (average user), fast (power
// user/stress test).
type SpeedMode string

const (
	// SpeedSlow simulates a careful or elderly user with
	// longer pauses between actions.
	SpeedSlow SpeedMode = "slow"

	// SpeedNormal simulates an average user with moderate
	// interaction timing.
	SpeedNormal SpeedMode = "normal"

	// SpeedFast simulates a power user or stress test with
	// minimal delays between actions.
	SpeedFast SpeedMode = "fast"
)

// SpeedConfig holds timing parameters for a speed mode.
// All durations define upper/lower bounds for randomized
// jitter that produces realistic human-like interaction.
type SpeedConfig struct {
	Mode          SpeedMode     // Speed preset name
	ClickDelayMin time.Duration // Minimum delay between clicks
	ClickDelayMax time.Duration // Maximum delay between clicks
	TypeDelayChar time.Duration // Delay per character when typing
	NavPause      time.Duration // Pause after navigation actions
	ScrollDelay   time.Duration // Delay per scroll step
}

// NewSpeedConfig creates a SpeedConfig with preset timing
// values for the given speed mode. Unknown modes default to
// SpeedNormal.
func NewSpeedConfig(mode SpeedMode) SpeedConfig {
	switch mode {
	case SpeedSlow:
		return SpeedConfig{
			Mode:          SpeedSlow,
			ClickDelayMin: 800 * time.Millisecond,
			ClickDelayMax: 2000 * time.Millisecond,
			TypeDelayChar: 150 * time.Millisecond,
			NavPause:      3 * time.Second,
			ScrollDelay:   500 * time.Millisecond,
		}
	case SpeedFast:
		return SpeedConfig{
			Mode:          SpeedFast,
			ClickDelayMin: 50 * time.Millisecond,
			ClickDelayMax: 150 * time.Millisecond,
			TypeDelayChar: 20 * time.Millisecond,
			NavPause:      300 * time.Millisecond,
			ScrollDelay:   50 * time.Millisecond,
		}
	default:
		return SpeedConfig{
			Mode:          SpeedNormal,
			ClickDelayMin: 200 * time.Millisecond,
			ClickDelayMax: 600 * time.Millisecond,
			TypeDelayChar: 60 * time.Millisecond,
			NavPause:      1 * time.Second,
			ScrollDelay:   150 * time.Millisecond,
		}
	}
}

// ClickDelay returns a randomized duration between
// ClickDelayMin and ClickDelayMax, simulating natural
// variation in human click timing.
func (c SpeedConfig) ClickDelay() time.Duration {
	if c.ClickDelayMax <= c.ClickDelayMin {
		return c.ClickDelayMin
	}
	spread := c.ClickDelayMax - c.ClickDelayMin
	jitter := time.Duration(
		rand.Int63n(int64(spread)),
	)
	return c.ClickDelayMin + jitter
}

// TypeChar sleeps for the per-character typing delay,
// respecting context cancellation. Returns an error if
// the context is cancelled during the sleep.
func (c SpeedConfig) TypeChar(ctx context.Context) error {
	return sleepWithContext(ctx, c.TypeDelayChar)
}

// AfterNavigation sleeps for the navigation pause duration,
// respecting context cancellation. Call this after page
// loads or URL changes.
func (c SpeedConfig) AfterNavigation(
	ctx context.Context,
) error {
	return sleepWithContext(ctx, c.NavPause)
}

// AfterScroll sleeps for the scroll delay duration,
// respecting context cancellation. Call this between
// scroll steps.
func (c SpeedConfig) AfterScroll(
	ctx context.Context,
) error {
	return sleepWithContext(ctx, c.ScrollDelay)
}

// AfterClick sleeps for a randomized click delay,
// respecting context cancellation. Call this between
// click actions.
func (c SpeedConfig) AfterClick(
	ctx context.Context,
) error {
	return sleepWithContext(ctx, c.ClickDelay())
}

// sleepWithContext pauses for the given duration, returning
// early with the context error if the context is cancelled.
func sleepWithContext(
	ctx context.Context, d time.Duration,
) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
