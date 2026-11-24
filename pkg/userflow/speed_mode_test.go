// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package userflow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpeedConfig_Slow(t *testing.T) {
	cfg := NewSpeedConfig(SpeedSlow)
	assert.Equal(t, SpeedSlow, cfg.Mode)
	assert.Equal(t, 800*time.Millisecond, cfg.ClickDelayMin)
	assert.Equal(t, 2000*time.Millisecond, cfg.ClickDelayMax)
	assert.Equal(t, 150*time.Millisecond, cfg.TypeDelayChar)
	assert.Equal(t, 3*time.Second, cfg.NavPause)
	assert.Equal(t, 500*time.Millisecond, cfg.ScrollDelay)
}

func TestNewSpeedConfig_Normal(t *testing.T) {
	cfg := NewSpeedConfig(SpeedNormal)
	assert.Equal(t, SpeedNormal, cfg.Mode)
	assert.Equal(t, 200*time.Millisecond, cfg.ClickDelayMin)
	assert.Equal(t, 600*time.Millisecond, cfg.ClickDelayMax)
	assert.Equal(t, 60*time.Millisecond, cfg.TypeDelayChar)
	assert.Equal(t, 1*time.Second, cfg.NavPause)
	assert.Equal(t, 150*time.Millisecond, cfg.ScrollDelay)
}

func TestNewSpeedConfig_Fast(t *testing.T) {
	cfg := NewSpeedConfig(SpeedFast)
	assert.Equal(t, SpeedFast, cfg.Mode)
	assert.Equal(t, 50*time.Millisecond, cfg.ClickDelayMin)
	assert.Equal(t, 150*time.Millisecond, cfg.ClickDelayMax)
	assert.Equal(t, 20*time.Millisecond, cfg.TypeDelayChar)
	assert.Equal(t, 300*time.Millisecond, cfg.NavPause)
	assert.Equal(t, 50*time.Millisecond, cfg.ScrollDelay)
}

func TestNewSpeedConfig_UnknownDefaultsToNormal(t *testing.T) {
	cfg := NewSpeedConfig(SpeedMode("turbo"))
	assert.Equal(t, SpeedNormal, cfg.Mode)
	assert.Equal(t, 200*time.Millisecond, cfg.ClickDelayMin)
}

func TestSpeedConfig_ClickDelay_WithinRange(t *testing.T) {
	cfg := NewSpeedConfig(SpeedNormal)
	for i := 0; i < 100; i++ {
		delay := cfg.ClickDelay()
		assert.GreaterOrEqual(
			t, delay, cfg.ClickDelayMin,
			"delay below minimum",
		)
		assert.Less(
			t, delay, cfg.ClickDelayMax,
			"delay at or above maximum",
		)
	}
}

func TestSpeedConfig_ClickDelay_SlowRange(t *testing.T) {
	cfg := NewSpeedConfig(SpeedSlow)
	for i := 0; i < 50; i++ {
		delay := cfg.ClickDelay()
		assert.GreaterOrEqual(
			t, delay, 800*time.Millisecond,
		)
		assert.Less(t, delay, 2000*time.Millisecond)
	}
}

func TestSpeedConfig_ClickDelay_FastRange(t *testing.T) {
	cfg := NewSpeedConfig(SpeedFast)
	for i := 0; i < 50; i++ {
		delay := cfg.ClickDelay()
		assert.GreaterOrEqual(
			t, delay, 50*time.Millisecond,
		)
		assert.Less(t, delay, 150*time.Millisecond)
	}
}

func TestSpeedConfig_ClickDelay_EqualMinMax(t *testing.T) {
	cfg := SpeedConfig{
		ClickDelayMin: 100 * time.Millisecond,
		ClickDelayMax: 100 * time.Millisecond,
	}
	delay := cfg.ClickDelay()
	assert.Equal(t, 100*time.Millisecond, delay)
}

func TestSpeedConfig_ClickDelay_MaxLessThanMin(t *testing.T) {
	cfg := SpeedConfig{
		ClickDelayMin: 200 * time.Millisecond,
		ClickDelayMax: 100 * time.Millisecond,
	}
	delay := cfg.ClickDelay()
	assert.Equal(t, 200*time.Millisecond, delay)
}

func TestSpeedConfig_TypeChar_Completes(t *testing.T) {
	cfg := NewSpeedConfig(SpeedFast)
	ctx := context.Background()
	start := time.Now()
	err := cfg.TypeChar(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(
		t, elapsed, cfg.TypeDelayChar-time.Millisecond,
	)
}

func TestSpeedConfig_TypeChar_ContextCancelled(
	t *testing.T,
) {
	cfg := NewSpeedConfig(SpeedSlow)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := cfg.TypeChar(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSpeedConfig_AfterNavigation_Completes(
	t *testing.T,
) {
	cfg := NewSpeedConfig(SpeedFast)
	ctx := context.Background()
	start := time.Now()
	err := cfg.AfterNavigation(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(
		t, elapsed, cfg.NavPause-time.Millisecond,
	)
}

func TestSpeedConfig_AfterNavigation_ContextCancelled(
	t *testing.T,
) {
	cfg := NewSpeedConfig(SpeedSlow)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cfg.AfterNavigation(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSpeedConfig_AfterScroll_Completes(t *testing.T) {
	cfg := NewSpeedConfig(SpeedFast)
	ctx := context.Background()
	start := time.Now()
	err := cfg.AfterScroll(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(
		t, elapsed, cfg.ScrollDelay-time.Millisecond,
	)
}

func TestSpeedConfig_AfterScroll_ContextCancelled(
	t *testing.T,
) {
	cfg := NewSpeedConfig(SpeedSlow)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cfg.AfterScroll(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSpeedConfig_AfterClick_Completes(t *testing.T) {
	cfg := NewSpeedConfig(SpeedFast)
	ctx := context.Background()
	err := cfg.AfterClick(ctx)
	assert.NoError(t, err)
}

func TestSpeedConfig_AfterClick_ContextCancelled(
	t *testing.T,
) {
	cfg := NewSpeedConfig(SpeedSlow)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cfg.AfterClick(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSleepWithContext_ZeroDuration(t *testing.T) {
	err := sleepWithContext(context.Background(), 0)
	assert.NoError(t, err)
}

func TestSleepWithContext_NegativeDuration(t *testing.T) {
	err := sleepWithContext(
		context.Background(), -1*time.Second,
	)
	assert.NoError(t, err)
}

func TestSleepWithContext_ShortDuration(t *testing.T) {
	start := time.Now()
	err := sleepWithContext(
		context.Background(), 10*time.Millisecond,
	)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 9*time.Millisecond)
}

func TestSleepWithContext_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Millisecond,
	)
	defer cancel()

	err := sleepWithContext(ctx, 10*time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestSpeedMode_Constants(t *testing.T) {
	assert.Equal(t, SpeedMode("slow"), SpeedSlow)
	assert.Equal(t, SpeedMode("normal"), SpeedNormal)
	assert.Equal(t, SpeedMode("fast"), SpeedFast)
}

func TestSpeedConfig_AllModes_ClickDelayMinLessThanMax(
	t *testing.T,
) {
	modes := []SpeedMode{SpeedSlow, SpeedNormal, SpeedFast}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			cfg := NewSpeedConfig(mode)
			assert.Less(
				t, cfg.ClickDelayMin, cfg.ClickDelayMax,
			)
		})
	}
}

func TestSpeedConfig_SlowIsSlowerThanNormal(t *testing.T) {
	slow := NewSpeedConfig(SpeedSlow)
	normal := NewSpeedConfig(SpeedNormal)

	assert.Greater(t, slow.ClickDelayMin, normal.ClickDelayMin)
	assert.Greater(t, slow.TypeDelayChar, normal.TypeDelayChar)
	assert.Greater(t, slow.NavPause, normal.NavPause)
}

func TestSpeedConfig_FastIsFasterThanNormal(t *testing.T) {
	fast := NewSpeedConfig(SpeedFast)
	normal := NewSpeedConfig(SpeedNormal)

	assert.Less(t, fast.ClickDelayMin, normal.ClickDelayMin)
	assert.Less(t, fast.TypeDelayChar, normal.TypeDelayChar)
	assert.Less(t, fast.NavPause, normal.NavPause)
}
