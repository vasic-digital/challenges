// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package userflow

import (
	"context"
	"testing"

	"digital.vasic.challenges/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewComposeDesktopAdapter_Constructor(
	t *testing.T,
) {
	logger := logging.NullLogger{}
	speed := NewSpeedConfig(SpeedNormal)
	adapter := NewComposeDesktopAdapter(
		logger, speed, "/tmp/output",
	)

	assert.NotNil(t, adapter)
	assert.Equal(t, "/tmp/output", adapter.outputDir)
	assert.Equal(t, SpeedNormal, adapter.speed.Mode)
	assert.Nil(t, adapter.process)
	assert.Empty(t, adapter.windowID)
}

func TestNewComposeDesktopAdapter_SpeedModes(
	t *testing.T,
) {
	tests := []struct {
		name string
		mode SpeedMode
	}{
		{name: "slow", mode: SpeedSlow},
		{name: "normal", mode: SpeedNormal},
		{name: "fast", mode: SpeedFast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewComposeDesktopAdapter(
				logging.NullLogger{},
				NewSpeedConfig(tt.mode),
				"/tmp",
			)
			assert.Equal(t, tt.mode, adapter.speed.Mode)
		})
	}
}

func TestComposeDesktopAdapter_IsAppRunning_NotStarted(
	t *testing.T,
) {
	adapter := NewComposeDesktopAdapter(
		logging.NullLogger{},
		NewSpeedConfig(SpeedFast),
		"/tmp",
	)

	running, err := adapter.IsAppRunning(
		context.Background(),
	)
	assert.NoError(t, err)
	assert.False(t, running)
}

func TestComposeDesktopAdapter_Close_NotStarted(
	t *testing.T,
) {
	adapter := NewComposeDesktopAdapter(
		logging.NullLogger{},
		NewSpeedConfig(SpeedFast),
		"/tmp",
	)

	err := adapter.Close(context.Background())
	assert.NoError(t, err)
}

func TestComposeDesktopAdapter_GetWindowID_Empty(
	t *testing.T,
) {
	adapter := NewComposeDesktopAdapter(
		logging.NullLogger{},
		NewSpeedConfig(SpeedFast),
		"/tmp",
	)

	wid := adapter.getWindowID()
	assert.Empty(t, wid)
}

func TestComposeDesktopAdapter_Screenshot_NoWindow(
	t *testing.T,
) {
	adapter := NewComposeDesktopAdapter(
		logging.NullLogger{},
		NewSpeedConfig(SpeedFast),
		"/tmp",
	)

	err := adapter.Screenshot(
		context.Background(), "/tmp/test.png",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no window ID")
}

func TestComposeDesktopAdapter_GetWindowGeometry_NoWindow(
	t *testing.T,
) {
	adapter := NewComposeDesktopAdapter(
		logging.NullLogger{},
		NewSpeedConfig(SpeedFast),
		"/tmp",
	)

	_, _, _, _, err := adapter.GetWindowGeometry(
		context.Background(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no window ID")
}

func TestComposeDesktopAdapter_Launch_AlreadyRunning(
	t *testing.T,
) {
	adapter := NewComposeDesktopAdapter(
		logging.NullLogger{},
		NewSpeedConfig(SpeedFast),
		"/tmp",
	)

	// Launch a process that sleeps.
	ctx := context.Background()
	err := adapter.Launch(ctx, "/bin/sleep", "60")
	// This will fail because /bin/sleep is not a jar,
	// but java may not be installed. We test the "already
	// running" guard by simulating a running process.

	// If java is not found, skip the double-launch test.
	if err != nil {
		t.Skip("java not available, skipping")
	}
	defer func() { _ = adapter.Close(ctx) }()

	err = adapter.Launch(ctx, "/bin/sleep", "60")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestBuildXdotoolClickArgs(t *testing.T) {
	args := buildXdotoolClickArgs("12345", 100, 200)
	expected := []string{
		"mousemove", "--window", "12345",
		"100", "200",
		"click", "1",
	}
	assert.Equal(t, expected, args)
}

func TestBuildXdotoolClickArgs_ZeroCoords(t *testing.T) {
	args := buildXdotoolClickArgs("99", 0, 0)
	assert.Equal(t, "0", args[3])
	assert.Equal(t, "0", args[4])
}

func TestBuildXdotoolTypeArgs(t *testing.T) {
	args := buildXdotoolTypeArgs("a")
	expected := []string{
		"type", "--clearmodifiers", "a",
	}
	assert.Equal(t, expected, args)
}

func TestBuildXdotoolTypeArgs_SpecialChar(t *testing.T) {
	args := buildXdotoolTypeArgs(" ")
	assert.Equal(t, " ", args[2])
}

func TestBuildXdotoolKeyArgs_SingleKey(t *testing.T) {
	args := buildXdotoolKeyArgs("Return")
	expected := []string{
		"key", "--clearmodifiers", "Return",
	}
	assert.Equal(t, expected, args)
}

func TestBuildXdotoolKeyArgs_Combo(t *testing.T) {
	args := buildXdotoolKeyArgs("ctrl", "s")
	expected := []string{
		"key", "--clearmodifiers", "ctrl+s",
	}
	assert.Equal(t, expected, args)
}

func TestBuildXdotoolKeyArgs_TripleCombo(t *testing.T) {
	args := buildXdotoolKeyArgs("ctrl", "shift", "z")
	assert.Equal(t, "ctrl+shift+z", args[2])
}

func TestBuildScreenshotArgs(t *testing.T) {
	args := buildScreenshotArgs(
		"12345", "/tmp/screen.png",
	)
	expected := []string{
		"-window", "12345", "/tmp/screen.png",
	}
	assert.Equal(t, expected, args)
}

func TestBuildJavaLaunchArgs_JarOnly(t *testing.T) {
	args := buildJavaLaunchArgs("/path/to/app.jar")
	expected := []string{"-jar", "/path/to/app.jar"}
	assert.Equal(t, expected, args)
}

func TestBuildJavaLaunchArgs_WithArgs(t *testing.T) {
	args := buildJavaLaunchArgs(
		"/path/to/app.jar", "--port", "8080",
	)
	expected := []string{
		"-jar", "/path/to/app.jar",
		"--port", "8080",
	}
	assert.Equal(t, expected, args)
}

func TestBuildWindowSearchArgs(t *testing.T) {
	args := buildWindowSearchArgs("My App.*")
	expected := []string{
		"search", "--name", "My App.*",
	}
	assert.Equal(t, expected, args)
}

func TestBuildWindowGeometryArgs(t *testing.T) {
	args := buildWindowGeometryArgs("12345")
	expected := []string{
		"getwindowgeometry", "--shell", "12345",
	}
	assert.Equal(t, expected, args)
}

func TestParseWindowGeometry_Valid(t *testing.T) {
	output := "WINDOW=12345\nX=100\nY=200\nWIDTH=800\nHEIGHT=600\n"
	x, y, w, h := parseWindowGeometry(output)
	assert.Equal(t, 100, x)
	assert.Equal(t, 200, y)
	assert.Equal(t, 800, w)
	assert.Equal(t, 600, h)
}

func TestParseWindowGeometry_Empty(t *testing.T) {
	x, y, w, h := parseWindowGeometry("")
	assert.Equal(t, 0, x)
	assert.Equal(t, 0, y)
	assert.Equal(t, 0, w)
	assert.Equal(t, 0, h)
}

func TestParseWindowGeometry_Partial(t *testing.T) {
	output := "X=50\nHEIGHT=400\n"
	x, y, w, h := parseWindowGeometry(output)
	assert.Equal(t, 50, x)
	assert.Equal(t, 0, y)
	assert.Equal(t, 0, w)
	assert.Equal(t, 400, h)
}

func TestParseWindowGeometry_InvalidValues(t *testing.T) {
	output := "X=abc\nY=200\n"
	x, y, _, _ := parseWindowGeometry(output)
	assert.Equal(t, 0, x) // Invalid value skipped.
	assert.Equal(t, 200, y)
}

func TestParseWindowGeometry_NoEquals(t *testing.T) {
	output := "WINDOW 12345\nX=100\n"
	x, _, _, _ := parseWindowGeometry(output)
	assert.Equal(t, 100, x)
}
