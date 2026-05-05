package userflow

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

// Compile-time interface check.
var _ BuildAdapter = (*CargoCLIAdapter)(nil)

func TestCargoCLIAdapter_Available_True(t *testing.T) {
	// Available now requires cargo in PATH.
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not in PATH, skipping")
	}

	dir := t.TempDir()
	toml := filepath.Join(dir, "Cargo.toml")
	err := os.WriteFile(
		toml, []byte("[package]\nname = \"test\"\n"), 0644,
	)
	require.NoError(t, err)

	adapter := NewCargoCLIAdapter(dir)
	assert.True(t, adapter.Available(context.Background()))
}

func TestCargoCLIAdapter_Available_False(t *testing.T) {
	dir := t.TempDir()
	adapter := NewCargoCLIAdapter(dir)
	assert.False(t, adapter.Available(context.Background()))
}

func TestCargoCLIAdapter_Constructor(t *testing.T) {
	adapter := NewCargoCLIAdapter("/tmp/rust-proj")
	assert.NotNil(t, adapter)
	assert.Equal(t, "/tmp/rust-proj", adapter.projectRoot)
}

func TestCargoCLIAdapter_ParseCargoTestJSON(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantTotal   int
		wantFailed  int
		wantSkipped int
	}{
		{
			name: "all_pass",
			output: `{"type":"test","event":"ok",` +
				`"name":"test_one"}` + "\n" +
				`{"type":"test","event":"ok",` +
				`"name":"test_two"}`,
			wantTotal:   2,
			wantFailed:  0,
			wantSkipped: 0,
		},
		{
			name: "mixed",
			output: `{"type":"test","event":"ok",` +
				`"name":"test_pass"}` + "\n" +
				`{"type":"test","event":"failed",` +
				`"name":"test_fail"}` + "\n" +
				`{"type":"test","event":"ignored",` +
				`"name":"test_skip"}`,
			wantTotal:   3,
			wantFailed:  1,
			wantSkipped: 1,
		},
		{
			name: "non_test_events_ignored",
			output: `{"type":"suite","event":"started",` +
				`"name":""}` + "\n" +
				`{"type":"test","event":"ok",` +
				`"name":"test_a"}`,
			wantTotal:   1,
			wantFailed:  0,
			wantSkipped: 0,
		},
		{
			name:        "empty_output",
			output:      "",
			wantTotal:   0,
			wantFailed:  0,
			wantSkipped: 0,
		},
		{
			name:        "invalid_json",
			output:      "not json at all",
			wantTotal:   0,
			wantFailed:  0,
			wantSkipped: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCargoTestJSON(
				tt.output, time.Second,
			)
			assert.Equal(
				t, tt.wantTotal, result.TotalTests,
			)
			assert.Equal(
				t, tt.wantFailed, result.TotalFailed,
			)
			assert.Equal(
				t, tt.wantSkipped, result.TotalSkipped,
			)
		})
	}
}

func TestCargoCLIAdapter_Build_NoCargo(t *testing.T) {
	dir := t.TempDir()
	adapter := NewCargoCLIAdapter(dir)

	result, err := adapter.Build(
		context.Background(),
		BuildTarget{Name: "desktop", Task: "build"},
	)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestCargoCLIAdapter_Lint_NoCargo(t *testing.T) {
	dir := t.TempDir()
	adapter := NewCargoCLIAdapter(dir)

	result, err := adapter.Lint(
		context.Background(),
		LintTarget{Name: "clippy", Task: "clippy"},
	)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "cargo clippy", result.Tool)
}
