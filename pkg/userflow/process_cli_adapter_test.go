package userflow

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ ProcessAdapter = (*ProcessCLIAdapter)(nil)

func TestProcessCLIAdapter_LaunchAndStop(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	adapter := NewProcessCLIAdapter()
	ctx := context.Background()

	err := adapter.Launch(ctx, ProcessConfig{
		Command: "sleep",
		Args:    []string{"60"},
	})
	require.NoError(t, err)

	// Wait for the process to start.
	err = adapter.WaitForReady(ctx, 2*time.Second)
	require.NoError(t, err)

	assert.True(t, adapter.IsRunning())

	// Stop should succeed.
	err = adapter.Stop()
	require.NoError(t, err)

	// Give a moment for cleanup.
	time.Sleep(100 * time.Millisecond)
	assert.False(t, adapter.IsRunning())
}

func TestProcessCLIAdapter_LaunchDuplicate(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	adapter := NewProcessCLIAdapter()
	ctx := context.Background()

	err := adapter.Launch(ctx, ProcessConfig{
		Command: "sleep",
		Args:    []string{"60"},
	})
	require.NoError(t, err)
	defer func() { _ = adapter.Stop() }()

	err = adapter.WaitForReady(ctx, 2*time.Second)
	require.NoError(t, err)

	// Second launch should fail.
	err = adapter.Launch(ctx, ProcessConfig{
		Command: "sleep",
		Args:    []string{"60"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestProcessCLIAdapter_IsRunning_NotStarted(t *testing.T) {
	adapter := NewProcessCLIAdapter()
	assert.False(t, adapter.IsRunning())
}

func TestProcessCLIAdapter_Stop_NotStarted(t *testing.T) {
	adapter := NewProcessCLIAdapter()
	err := adapter.Stop()
	assert.NoError(t, err)
}

func TestProcessCLIAdapter_WaitForReady_Timeout(t *testing.T) {
	adapter := NewProcessCLIAdapter()
	ctx := context.Background()

	// Launch a command that exits immediately.
	err := adapter.Launch(ctx, ProcessConfig{
		Command: "true",
	})
	require.NoError(t, err)

	// Give it time to exit.
	time.Sleep(200 * time.Millisecond)

	err = adapter.WaitForReady(
		ctx, 500*time.Millisecond,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestProcessCLIAdapter_WaitForReady_ContextCancel(
	t *testing.T,
) {
	adapter := NewProcessCLIAdapter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := adapter.WaitForReady(ctx, 5*time.Second)
	assert.Error(t, err)
}

func TestProcessCLIAdapter_LaunchWithEnv(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	adapter := NewProcessCLIAdapter()
	ctx := context.Background()

	err := adapter.Launch(ctx, ProcessConfig{
		Command: "sleep",
		Args:    []string{"60"},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	})
	require.NoError(t, err)
	defer func() { _ = adapter.Stop() }()

	err = adapter.WaitForReady(ctx, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, adapter.IsRunning())
}
