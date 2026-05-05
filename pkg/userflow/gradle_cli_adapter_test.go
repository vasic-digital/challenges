package userflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ BuildAdapter = (*GradleCLIAdapter)(nil)

func TestGradleCLIAdapter_Available_True(t *testing.T) {
	// Available now requires java in PATH.
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not in PATH, skipping")
	}

	dir := t.TempDir()
	gradlew := filepath.Join(dir, "gradlew")
	err := os.WriteFile(gradlew, []byte("#!/bin/sh"), 0755)
	require.NoError(t, err)

	adapter := NewGradleCLIAdapter(dir, false)
	assert.True(t, adapter.Available(context.Background()))
}

func TestGradleCLIAdapter_Available_False(t *testing.T) {
	dir := t.TempDir()
	adapter := NewGradleCLIAdapter(dir, false)
	assert.False(t, adapter.Available(context.Background()))
}

func TestGradleCLIAdapter_Constructor(t *testing.T) {
	tests := []struct {
		name         string
		projectRoot  string
		useContainer bool
	}{
		{
			name:         "local",
			projectRoot:  "/tmp/project",
			useContainer: false,
		},
		{
			name:         "container",
			projectRoot:  "/tmp/project",
			useContainer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewGradleCLIAdapter(
				tt.projectRoot, tt.useContainer,
			)
			assert.NotNil(t, adapter)
			assert.Equal(
				t, tt.projectRoot, adapter.projectRoot,
			)
			assert.Equal(
				t, tt.useContainer, adapter.useContainer,
			)
		})
	}
}

func TestGradleCLIAdapter_Build_NoGradlew(t *testing.T) {
	dir := t.TempDir()
	adapter := NewGradleCLIAdapter(dir, false)

	result, err := adapter.Build(
		context.Background(),
		BuildTarget{
			Name: "app", Task: "assembleDebug",
		},
	)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "app", result.Target)
}

func TestGradleCLIAdapter_RunTests_NoGradlew(t *testing.T) {
	dir := t.TempDir()
	adapter := NewGradleCLIAdapter(dir, false)

	result, err := adapter.RunTests(
		context.Background(),
		TestTarget{Name: "unit", Task: "test"},
	)

	assert.Error(t, err)
	assert.NotNil(t, result)
}

func TestGradleCLIAdapter_Lint_NoGradlew(t *testing.T) {
	dir := t.TempDir()
	adapter := NewGradleCLIAdapter(dir, false)

	result, err := adapter.Lint(
		context.Background(),
		LintTarget{Name: "lint", Task: "lint"},
	)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "gradle:lint", result.Tool)
}

func TestGradleCLIAdapter_Build_WithScript(t *testing.T) {
	dir := t.TempDir()
	// Create a mock gradlew that succeeds.
	script := filepath.Join(dir, "gradlew")
	err := os.WriteFile(
		script,
		[]byte("#!/bin/sh\necho BUILD OK\n"),
		0755,
	)
	require.NoError(t, err)

	adapter := NewGradleCLIAdapter(dir, false)
	result, err := adapter.Build(
		context.Background(),
		BuildTarget{
			Name: "app",
			Task: "assembleDebug",
		},
	)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "BUILD OK")
}

func TestGradleCLIAdapter_Lint_WithScript(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gradlew")
	err := os.WriteFile(
		script,
		[]byte("#!/bin/sh\necho LINT OK\n"),
		0755,
	)
	require.NoError(t, err)

	adapter := NewGradleCLIAdapter(dir, false)
	result, err := adapter.Lint(
		context.Background(),
		LintTarget{Name: "lint", Task: "lint"},
	)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "LINT OK")
}
