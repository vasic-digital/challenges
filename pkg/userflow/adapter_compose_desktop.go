// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package userflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"digital.vasic.challenges/pkg/logging"
)

// ComposeDesktopAdapter automates desktop applications by
// launching them as a subprocess and using system-level tools
// (xdotool on Linux) for real mouse clicks, keyboard input,
// and window management.
//
// This adapter provides real hardware-level interaction — not
// simulated events. Every click moves the actual mouse cursor,
// every keystroke goes through the OS input pipeline.
type ComposeDesktopAdapter struct {
	logger    logging.Logger
	speed     SpeedConfig
	process   *exec.Cmd
	done      chan struct{}
	windowID  string
	outputDir string
	mu        sync.Mutex
}

// NewComposeDesktopAdapter creates a ComposeDesktopAdapter
// with the given logger, speed configuration, and output
// directory for screenshots.
func NewComposeDesktopAdapter(
	logger logging.Logger,
	speed SpeedConfig,
	outputDir string,
) *ComposeDesktopAdapter {
	return &ComposeDesktopAdapter{
		logger:    logger,
		speed:     speed,
		outputDir: outputDir,
	}
}

// Launch starts a desktop application by running
// `java -jar <jarPath>` with the given arguments as a
// background process.
func (a *ComposeDesktopAdapter) Launch(
	ctx context.Context,
	jarPath string,
	args ...string,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.process != nil {
		return fmt.Errorf("app already running")
	}

	cmdArgs := append([]string{"-jar", jarPath}, args...)
	cmd := exec.CommandContext(ctx, "java", cmdArgs...)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch app: %w", err)
	}

	a.process = cmd
	a.done = make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(a.done)
	}()

	a.logger.Info("launched desktop app",
		logging.StringField("jar", jarPath),
	)
	return nil
}

// WaitForWindow polls for a window matching the given title
// pattern using xdotool --name search. Returns an error if
// the window is not found within the timeout.
func (a *ComposeDesktopAdapter) WaitForWindow(
	ctx context.Context,
	titlePattern string,
	timeout time.Duration,
) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		cmd := exec.CommandContext(
			ctx, "xdotool", "search", "--name",
			titlePattern,
		)
		out, err := cmd.Output()
		if err == nil {
			windowID := strings.TrimSpace(string(out))
			if windowID != "" {
				// Take the first window ID if multiple
				// are returned.
				lines := strings.Split(windowID, "\n")
				a.mu.Lock()
				a.windowID = lines[0]
				a.mu.Unlock()
				a.logger.Info("found window",
					logging.StringField("id", lines[0]),
					logging.StringField(
						"pattern", titlePattern,
					),
				)
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"wait for window: %w", ctx.Err(),
			)
		case <-deadline:
			return fmt.Errorf(
				"wait for window %q: timed out after %s",
				titlePattern, timeout,
			)
		case <-ticker.C:
		}
	}
}

// Click performs a real mouse click at the given coordinates
// relative to the window using xdotool.
func (a *ComposeDesktopAdapter) Click(
	ctx context.Context, x, y int,
) error {
	if err := a.activateWindow(ctx); err != nil {
		return fmt.Errorf("click activate: %w", err)
	}

	// Move mouse to window-relative position and click.
	cmd := exec.CommandContext(
		ctx, "xdotool",
		"mousemove", "--window", a.getWindowID(),
		strconv.Itoa(x), strconv.Itoa(y),
		"click", "1",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("click at (%d,%d): %w", x, y, err)
	}

	return a.speed.AfterClick(ctx)
}

// DoubleClick performs a real double-click at the given
// coordinates relative to the window.
func (a *ComposeDesktopAdapter) DoubleClick(
	ctx context.Context, x, y int,
) error {
	if err := a.activateWindow(ctx); err != nil {
		return fmt.Errorf("double click activate: %w", err)
	}

	cmd := exec.CommandContext(
		ctx, "xdotool",
		"mousemove", "--window", a.getWindowID(),
		strconv.Itoa(x), strconv.Itoa(y),
		"click", "--repeat", "2", "--delay", "50", "1",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"double click at (%d,%d): %w", x, y, err,
		)
	}

	return a.speed.AfterClick(ctx)
}

// TypeText types the given text using xdotool, with a
// per-character delay defined by the speed configuration
// for realistic human-like input.
func (a *ComposeDesktopAdapter) TypeText(
	ctx context.Context, text string,
) error {
	if err := a.activateWindow(ctx); err != nil {
		return fmt.Errorf("type text activate: %w", err)
	}

	for _, ch := range text {
		cmd := exec.CommandContext(
			ctx, "xdotool", "type", "--clearmodifiers",
			string(ch),
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(
				"type char %q: %w", string(ch), err,
			)
		}
		if err := a.speed.TypeChar(ctx); err != nil {
			return fmt.Errorf("type delay: %w", err)
		}
	}

	return nil
}

// KeyCombo sends a keyboard shortcut using xdotool. Keys
// are specified as xdotool key names, e.g., "ctrl+s",
// "alt+F4", "Return".
func (a *ComposeDesktopAdapter) KeyCombo(
	ctx context.Context, keys ...string,
) error {
	if err := a.activateWindow(ctx); err != nil {
		return fmt.Errorf("key combo activate: %w", err)
	}

	combo := strings.Join(keys, "+")
	cmd := exec.CommandContext(
		ctx, "xdotool", "key", "--clearmodifiers", combo,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("key combo %q: %w", combo, err)
	}

	return a.speed.AfterClick(ctx)
}

// Screenshot captures the window as a PNG image at the given
// path using ImageMagick's import command. If the path is
// empty, a timestamped filename in the output directory is
// used.
func (a *ComposeDesktopAdapter) Screenshot(
	ctx context.Context, path string,
) error {
	if path == "" {
		path = fmt.Sprintf(
			"%s/screenshot_%d.png",
			a.outputDir, time.Now().UnixMilli(),
		)
	}

	wid := a.getWindowID()
	if wid == "" {
		return fmt.Errorf("screenshot: no window ID")
	}

	// Use ImageMagick import to capture a specific window.
	cmd := exec.CommandContext(
		ctx, "import", "-window", wid, path,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}

	a.logger.Info("screenshot captured",
		logging.StringField("path", path),
	)
	return nil
}

// Close gracefully closes the application by sending
// alt+F4, then falls back to killing the process if it
// does not exit within 5 seconds.
func (a *ComposeDesktopAdapter) Close(
	ctx context.Context,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.process == nil || a.process.Process == nil {
		return nil
	}

	// Try graceful close via alt+F4.
	if a.windowID != "" {
		closeCmd := exec.CommandContext(
			ctx, "xdotool", "key", "--window",
			a.windowID, "alt+F4",
		)
		_ = closeCmd.Run()

		// Wait up to 5 seconds for graceful exit.
		graceful := time.After(5 * time.Second)
		select {
		case <-a.done:
			a.logger.Info("app closed gracefully")
			a.process = nil
			a.windowID = ""
			return nil
		case <-graceful:
		}
	}

	// Force kill.
	if err := a.process.Process.Kill(); err != nil {
		return fmt.Errorf("kill process: %w", err)
	}
	<-a.done

	a.logger.Info("app force killed")
	a.process = nil
	a.windowID = ""
	return nil
}

// GetWindowGeometry returns the position and size of the
// current window using xdotool getwindowgeometry.
func (a *ComposeDesktopAdapter) GetWindowGeometry(
	ctx context.Context,
) (x, y, w, h int, err error) {
	wid := a.getWindowID()
	if wid == "" {
		return 0, 0, 0, 0, fmt.Errorf(
			"get geometry: no window ID",
		)
	}

	// Get position.
	posCmd := exec.CommandContext(
		ctx, "xdotool", "getwindowgeometry", "--shell",
		wid,
	)
	posOut, err := posCmd.Output()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf(
			"get geometry: %w", err,
		)
	}

	x, y, w, h = parseWindowGeometry(string(posOut))
	return x, y, w, h, nil
}

// Available returns true if xdotool is installed and can be
// found in PATH.
func (a *ComposeDesktopAdapter) Available(
	ctx context.Context,
) bool {
	_, err := exec.LookPath("xdotool")
	return err == nil
}

// IsAppRunning returns true if the application process is
// still running.
func (a *ComposeDesktopAdapter) IsAppRunning(
	ctx context.Context,
) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.process == nil || a.process.Process == nil {
		return false, nil
	}

	select {
	case <-a.done:
		return false, nil
	default:
		return true, nil
	}
}

// activateWindow brings the window to the foreground using
// xdotool windowactivate.
func (a *ComposeDesktopAdapter) activateWindow(
	ctx context.Context,
) error {
	wid := a.getWindowID()
	if wid == "" {
		return fmt.Errorf("no window ID set")
	}

	cmd := exec.CommandContext(
		ctx, "xdotool", "windowactivate", "--sync", wid,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("activate window: %w", err)
	}
	return nil
}

// getWindowID returns the current window ID in a thread-safe
// manner.
func (a *ComposeDesktopAdapter) getWindowID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windowID
}

// parseWindowGeometry parses xdotool --shell output into
// position and size values. The output format is:
//
//	WINDOW=12345
//	X=100
//	Y=200
//	WIDTH=800
//	HEIGHT=600
func parseWindowGeometry(output string) (x, y, w, h int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		n, err := strconv.Atoi(val)
		if err != nil {
			continue
		}
		switch key {
		case "X":
			x = n
		case "Y":
			y = n
		case "WIDTH":
			w = n
		case "HEIGHT":
			h = n
		}
	}
	return x, y, w, h
}

// buildXdotoolClickArgs constructs the argument list for an
// xdotool click command. Exported for testing command
// construction.
func buildXdotoolClickArgs(
	windowID string, x, y int,
) []string {
	return []string{
		"mousemove", "--window", windowID,
		strconv.Itoa(x), strconv.Itoa(y),
		"click", "1",
	}
}

// buildXdotoolTypeArgs constructs the argument list for an
// xdotool type command for a single character.
func buildXdotoolTypeArgs(ch string) []string {
	return []string{"type", "--clearmodifiers", ch}
}

// buildXdotoolKeyArgs constructs the argument list for an
// xdotool key combo command.
func buildXdotoolKeyArgs(keys ...string) []string {
	combo := strings.Join(keys, "+")
	return []string{"key", "--clearmodifiers", combo}
}

// buildScreenshotArgs constructs the argument list for an
// ImageMagick import screenshot command.
func buildScreenshotArgs(
	windowID, path string,
) []string {
	return []string{"-window", windowID, path}
}

// buildJavaLaunchArgs constructs the argument list for
// launching a Java application.
func buildJavaLaunchArgs(
	jarPath string, args ...string,
) []string {
	return append([]string{"-jar", jarPath}, args...)
}

// buildWindowSearchArgs constructs the argument list for
// xdotool window search by title pattern.
func buildWindowSearchArgs(titlePattern string) []string {
	return []string{"search", "--name", titlePattern}
}

// buildWindowGeometryArgs constructs the argument list for
// xdotool getwindowgeometry.
func buildWindowGeometryArgs(windowID string) []string {
	return []string{"getwindowgeometry", "--shell", windowID}
}
