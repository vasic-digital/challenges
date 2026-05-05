package panoptic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIAdapter_SetWorkDir(t *testing.T) {
	adapter := NewCLIAdapter("/bin/test")
	adapter.SetWorkDir("/tmp/work")
	assert.Equal(t, "/tmp/work", adapter.workDir)
}

func TestCLIAdapter_SetEnv(t *testing.T) {
	adapter := NewCLIAdapter("/bin/test")
	adapter.SetEnv("FOO", "bar")
	assert.Equal(t, "bar", adapter.env["FOO"])
}

func TestCLIAdapter_Available_Missing(t *testing.T) {
	adapter := NewCLIAdapter("/nonexistent/panoptic")
	assert.False(t, adapter.Available(context.Background()))
}

func TestCLIAdapter_Available_Exists(t *testing.T) {
	// Create a temporary executable file.
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "panoptic")
	err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755)
	require.NoError(t, err)

	adapter := NewCLIAdapter(binPath)
	assert.True(t, adapter.Available(context.Background()))
}

func TestCLIAdapter_Available_Directory(t *testing.T) {
	adapter := NewCLIAdapter(t.TempDir())
	assert.False(t, adapter.Available(context.Background()))
}

func TestCLIAdapter_Run_Success(t *testing.T) {
	// Override commandFunc to use echo.
	origCmd := commandFunc
	defer func() { commandFunc = origCmd }()

	commandFunc = func(
		ctx context.Context,
		name string,
		args ...string,
	) *exec.Cmd {
		return exec.CommandContext(
			ctx, "echo", "panoptic output",
		)
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(configPath, []byte(
		"name: test\noutput: "+tmpDir+"\n",
	), 0o644)
	require.NoError(t, err)

	adapter := NewCLIAdapter("/bin/echo")
	result, err := adapter.Run(
		context.Background(), configPath,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "panoptic output")
}

func TestCLIAdapter_Run_WithOptions(t *testing.T) {
	origCmd := commandFunc
	defer func() { commandFunc = origCmd }()

	var capturedArgs []string
	commandFunc = func(
		ctx context.Context,
		name string,
		args ...string,
	) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(configPath, []byte("name: t\n"), 0o644)
	require.NoError(t, err)

	adapter := NewCLIAdapter("/bin/panoptic")
	_, err = adapter.Run(
		context.Background(), configPath,
		RunWithVerbose(),
		RunWithOutputDir("/tmp/out"),
		RunWithTimeout(5*time.Minute),
	)
	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "--verbose")
	assert.Contains(t, capturedArgs, "--output")
	assert.Contains(t, capturedArgs, "/tmp/out")
}

func TestCLIAdapter_Run_Failure(t *testing.T) {
	origCmd := commandFunc
	defer func() { commandFunc = origCmd }()

	commandFunc = func(
		ctx context.Context,
		name string,
		args ...string,
	) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	adapter := NewCLIAdapter("/bin/false")
	result, err := adapter.Run(
		context.Background(), "nonexistent.yaml",
	)
	require.NoError(t, err) // Non-zero exit is not an error
	assert.NotEqual(t, 0, result.ExitCode)
}

func TestCLIAdapter_Version(t *testing.T) {
	origCmd := commandFunc
	defer func() { commandFunc = origCmd }()

	commandFunc = func(
		ctx context.Context,
		name string,
		args ...string,
	) *exec.Cmd {
		return exec.CommandContext(
			ctx, "echo", "panoptic v1.2.3",
		)
	}

	adapter := NewCLIAdapter("/bin/panoptic")
	version, err := adapter.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "panoptic v1.2.3", version)
}

func TestCLIAdapter_ScanArtifacts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock artifacts.
	screenshotDir := filepath.Join(tmpDir, "screenshots")
	require.NoError(t, os.MkdirAll(screenshotDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(screenshotDir, "login.png"),
		[]byte("png"), 0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "session.mp4"),
		[]byte("mp4"), 0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "report.html"),
		[]byte("<html>"), 0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "report.json"),
		[]byte("[]"), 0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "ai_error_report.json"),
		[]byte("{}"), 0o644,
	))

	adapter := NewCLIAdapter("/bin/test")
	result := &PanopticRunResult{}
	adapter.scanArtifacts(tmpDir, result)

	assert.Len(t, result.Screenshots, 1)
	assert.Len(t, result.Videos, 1)
	assert.NotEmpty(t, result.ReportHTML)
	assert.NotEmpty(t, result.ReportJSON)
	assert.NotEmpty(t, result.AIErrorReport)
}

func TestCLIAdapter_GuessOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte(
		"name: test\noutput: \"./reports/qa/test\"\n",
	), 0o644)
	require.NoError(t, err)

	adapter := NewCLIAdapter("/bin/test")
	dir := adapter.guessOutputDir(configPath)
	assert.Equal(t, "./reports/qa/test", dir)
}

func TestCLIAdapter_GuessOutputDir_NoOutput(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte(
		"name: test\n",
	), 0o644)
	require.NoError(t, err)

	adapter := NewCLIAdapter("/bin/test")
	dir := adapter.guessOutputDir(configPath)
	assert.Empty(t, dir)
}
