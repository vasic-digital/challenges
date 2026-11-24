// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package userflow

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"

	"digital.vasic.challenges/pkg/logging"
)

// FFmpegRecorderAdapter records a screen region to a video
// file using ffmpeg. Supports Linux (x11grab) and can be
// extended for other platforms. Records at configurable FPS
// and resolution.
type FFmpegRecorderAdapter struct {
	logger   logging.Logger
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	filePath string
	fps      int
	display  string // e.g., ":0" for X11
	mu       sync.Mutex
}

// NewFFmpegRecorderAdapter creates an FFmpegRecorderAdapter
// that will write to the given output path at the given FPS.
// If display is empty, it defaults to ":0".
func NewFFmpegRecorderAdapter(
	logger logging.Logger,
	outputPath string,
	fps int,
) *FFmpegRecorderAdapter {
	if fps <= 0 {
		fps = 30
	}
	return &FFmpegRecorderAdapter{
		logger:   logger,
		filePath: outputPath,
		fps:      fps,
		display:  ":0",
	}
}

// SetDisplay overrides the X11 display string (e.g., ":1").
func (a *FFmpegRecorderAdapter) SetDisplay(display string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.display = display
}

// StartRegion starts recording a rectangular screen region.
// The coordinates (x, y) define the top-left corner, and
// (w, h) define the width and height in pixels.
func (a *FFmpegRecorderAdapter) StartRegion(
	ctx context.Context, x, y, w, h int,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd != nil {
		return fmt.Errorf("recording already in progress")
	}

	args := buildFFmpegRegionArgs(
		a.display, x, y, w, h, a.fps, a.filePath,
	)
	return a.startProcess(ctx, args)
}

// StartFullScreen starts recording the entire screen.
func (a *FFmpegRecorderAdapter) StartFullScreen(
	ctx context.Context,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd != nil {
		return fmt.Errorf("recording already in progress")
	}

	args := buildFFmpegFullScreenArgs(
		a.display, a.fps, a.filePath,
	)
	return a.startProcess(ctx, args)
}

// Stop gracefully stops the recording by sending 'q' to
// ffmpeg's stdin. If the process does not exit, it is
// killed.
func (a *FFmpegRecorderAdapter) Stop(
	ctx context.Context,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd == nil {
		return fmt.Errorf("no recording in progress")
	}

	// Send 'q' to ffmpeg stdin for graceful stop.
	if a.stdin != nil {
		_, _ = a.stdin.Write([]byte("q"))
		_ = a.stdin.Close()
	}

	// Wait for the process to exit.
	err := a.cmd.Wait()
	a.cmd = nil
	a.stdin = nil

	// ffmpeg exits with non-zero when stopped via 'q',
	// which is expected behavior.
	if err != nil {
		a.logger.Debug("ffmpeg exited",
			logging.StringField("status", err.Error()),
		)
	}

	a.logger.Info("recording stopped",
		logging.StringField("path", a.filePath),
	)
	return nil
}

// IsRecording returns true if a recording is currently in
// progress.
func (a *FFmpegRecorderAdapter) IsRecording() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cmd != nil
}

// GetFilePath returns the output file path.
func (a *FFmpegRecorderAdapter) GetFilePath() string {
	return a.filePath
}

// Available returns true if ffmpeg is installed and can be
// found in PATH.
func (a *FFmpegRecorderAdapter) Available(
	ctx context.Context,
) bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// startProcess launches ffmpeg with the given arguments and
// captures stdin for later stop signaling. Must be called
// with mu held.
func (a *FFmpegRecorderAdapter) startProcess(
	ctx context.Context, args []string,
) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	a.cmd = cmd
	a.stdin = stdin

	a.logger.Info("recording started",
		logging.StringField("path", a.filePath),
		logging.IntField("fps", a.fps),
	)
	return nil
}

// buildFFmpegRegionArgs constructs the ffmpeg argument list
// for recording a specific screen region using x11grab.
func buildFFmpegRegionArgs(
	display string, x, y, w, h, fps int, output string,
) []string {
	videoSize := strconv.Itoa(w) + "x" + strconv.Itoa(h)
	input := fmt.Sprintf(
		"%s+%d,%d", display, x, y,
	)
	return []string{
		"-y",
		"-f", "x11grab",
		"-framerate", strconv.Itoa(fps),
		"-video_size", videoSize,
		"-i", input,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		output,
	}
}

// buildFFmpegFullScreenArgs constructs the ffmpeg argument
// list for recording the full screen using x11grab.
func buildFFmpegFullScreenArgs(
	display string, fps int, output string,
) []string {
	return []string{
		"-y",
		"-f", "x11grab",
		"-framerate", strconv.Itoa(fps),
		"-i", display,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		output,
	}
}
