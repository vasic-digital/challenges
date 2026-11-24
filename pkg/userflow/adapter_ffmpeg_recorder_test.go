// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package userflow

import (
	"context"
	"os/exec"
	"testing"

	"digital.vasic.challenges/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFFmpegRecorderAdapter_Constructor(
	t *testing.T,
) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 30,
	)
	assert.NotNil(t, adapter)
	assert.Equal(t, "/tmp/out.mp4", adapter.filePath)
	assert.Equal(t, 30, adapter.fps)
	assert.Equal(t, ":0", adapter.display)
	assert.Nil(t, adapter.cmd)
	assert.False(t, adapter.IsRecording())
}

func TestNewFFmpegRecorderAdapter_DefaultFPS(t *testing.T) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 0,
	)
	assert.Equal(t, 30, adapter.fps)
}

func TestNewFFmpegRecorderAdapter_NegativeFPS(
	t *testing.T,
) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", -1,
	)
	assert.Equal(t, 30, adapter.fps)
}

func TestFFmpegRecorderAdapter_SetDisplay(t *testing.T) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 30,
	)
	adapter.SetDisplay(":1")
	assert.Equal(t, ":1", adapter.display)
}

func TestFFmpegRecorderAdapter_GetFilePath(t *testing.T) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/recording.mp4", 24,
	)
	assert.Equal(
		t, "/tmp/recording.mp4", adapter.GetFilePath(),
	)
}

func TestFFmpegRecorderAdapter_IsRecording_Initial(
	t *testing.T,
) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 30,
	)
	assert.False(t, adapter.IsRecording())
}

func TestFFmpegRecorderAdapter_Stop_NoRecording(
	t *testing.T,
) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 30,
	)
	err := adapter.Stop(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recording")
}

func TestFFmpegRecorderAdapter_StartRegion_AlreadyRecording(
	t *testing.T,
) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 30,
	)
	// Simulate in-progress recording with a non-nil cmd.
	adapter.mu.Lock()
	adapter.cmd = exec.Command("true")
	adapter.mu.Unlock()

	err := adapter.StartRegion(
		context.Background(), 0, 0, 1920, 1080,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	// Clean up.
	adapter.mu.Lock()
	adapter.cmd = nil
	adapter.mu.Unlock()
}

func TestFFmpegRecorderAdapter_StartFullScreen_AlreadyRecording(
	t *testing.T,
) {
	adapter := NewFFmpegRecorderAdapter(
		logging.NullLogger{}, "/tmp/out.mp4", 30,
	)
	adapter.mu.Lock()
	adapter.cmd = exec.Command("true")
	adapter.mu.Unlock()

	err := adapter.StartFullScreen(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	adapter.mu.Lock()
	adapter.cmd = nil
	adapter.mu.Unlock()
}

func TestBuildFFmpegRegionArgs(t *testing.T) {
	args := buildFFmpegRegionArgs(
		":0", 100, 200, 1920, 1080, 30,
		"/tmp/out.mp4",
	)
	expected := []string{
		"-y",
		"-f", "x11grab",
		"-framerate", "30",
		"-video_size", "1920x1080",
		"-i", ":0+100,200",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"/tmp/out.mp4",
	}
	assert.Equal(t, expected, args)
}

func TestBuildFFmpegRegionArgs_ZeroOffset(t *testing.T) {
	args := buildFFmpegRegionArgs(
		":0", 0, 0, 800, 600, 24,
		"/tmp/test.mp4",
	)
	assert.Equal(t, ":0+0,0", args[8])
	assert.Equal(t, "800x600", args[6])
	assert.Equal(t, "24", args[4])
}

func TestBuildFFmpegRegionArgs_CustomDisplay(t *testing.T) {
	args := buildFFmpegRegionArgs(
		":1", 50, 75, 640, 480, 60,
		"/tmp/display1.mp4",
	)
	assert.Equal(t, ":1+50,75", args[8])
}

func TestBuildFFmpegFullScreenArgs(t *testing.T) {
	args := buildFFmpegFullScreenArgs(
		":0", 30, "/tmp/full.mp4",
	)
	expected := []string{
		"-y",
		"-f", "x11grab",
		"-framerate", "30",
		"-i", ":0",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"/tmp/full.mp4",
	}
	assert.Equal(t, expected, args)
}

func TestBuildFFmpegFullScreenArgs_CustomFPS(t *testing.T) {
	args := buildFFmpegFullScreenArgs(
		":0", 60, "/tmp/60fps.mp4",
	)
	assert.Equal(t, "60", args[4])
}

func TestBuildFFmpegFullScreenArgs_OutputPath(
	t *testing.T,
) {
	args := buildFFmpegFullScreenArgs(
		":0", 30, "/home/user/video.mp4",
	)
	assert.Equal(
		t, "/home/user/video.mp4",
		args[len(args)-1],
	)
}
