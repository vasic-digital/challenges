// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package userflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.challenges/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordingValidator_Validate_NonExistentFile(
	t *testing.T,
) {
	v := NewRecordingValidator(logging.NullLogger{})
	result, err := v.Validate(
		context.Background(),
		"/nonexistent/path/video.mp4",
	)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Equal(t, int64(0), result.FileSize)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "file not found")
}

func TestRecordingValidator_Validate_EmptyFile(
	t *testing.T,
) {
	// Create a temp empty file.
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.mp4")
	err := os.WriteFile(emptyFile, []byte{}, 0o644)
	require.NoError(t, err)

	v := NewRecordingValidator(logging.NullLogger{})
	result, err := v.Validate(
		context.Background(), emptyFile,
	)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Equal(t, int64(0), result.FileSize)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "empty")
}

func TestValidationResult_Structure(t *testing.T) {
	result := ValidationResult{
		FileSize:       1024,
		Duration:       5.5,
		FrameCount:     165,
		HasBlackFrames: false,
		IsValid:        true,
		Errors:         []string{},
	}

	assert.Equal(t, int64(1024), result.FileSize)
	assert.Equal(t, 5.5, result.Duration)
	assert.Equal(t, 165, result.FrameCount)
	assert.False(t, result.HasBlackFrames)
	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)
}

func TestValidationResult_WithErrors(t *testing.T) {
	result := ValidationResult{
		FileSize:       0,
		Duration:       0,
		FrameCount:     0,
		HasBlackFrames: true,
		IsValid:        false,
		Errors:         []string{"file not found", "duration is zero"},
	}

	assert.False(t, result.IsValid)
	assert.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0], "file not found")
	assert.Contains(t, result.Errors[1], "duration is zero")
}

func TestBuildDurationProbeArgs(t *testing.T) {
	args := buildDurationProbeArgs("/tmp/video.mp4")
	expected := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		"/tmp/video.mp4",
	}
	assert.Equal(t, expected, args)
}

func TestBuildFrameCountProbeArgs(t *testing.T) {
	args := buildFrameCountProbeArgs("/tmp/video.mp4")
	expected := []string{
		"-v", "error",
		"-count_frames",
		"-select_streams", "v:0",
		"-show_entries", "stream=nb_read_frames",
		"-of", "csv=p=0",
		"/tmp/video.mp4",
	}
	assert.Equal(t, expected, args)
}

func TestBuildBlackDetectArgs(t *testing.T) {
	args := buildBlackDetectArgs("/tmp/video.mp4")
	expected := []string{
		"-i", "/tmp/video.mp4",
		"-vf", "blackdetect=d=0.5:pix_th=0.1",
		"-an",
		"-f", "null",
		"-",
	}
	assert.Equal(t, expected, args)
}

func TestBuildThumbnailArgs(t *testing.T) {
	args := buildThumbnailArgs(
		"/tmp/video.mp4", "/tmp/thumbs", 5,
	)
	assert.Equal(t, "-i", args[0])
	assert.Equal(t, "/tmp/video.mp4", args[1])
	assert.Contains(t, args[2], "-vf")
	// Output pattern should include thumb_%04d.png.
	pattern := filepath.Join(
		"/tmp/thumbs", "thumb_%04d.png",
	)
	assert.Equal(t, pattern, args[len(args)-1])
}

func TestBuildThumbnailArgs_SingleFrame(t *testing.T) {
	args := buildThumbnailArgs(
		"/tmp/video.mp4", "/tmp/thumbs", 1,
	)
	assert.Contains(t, args, "-frames:v")
	// Find the index of -frames:v and check its value.
	for i, arg := range args {
		if arg == "-frames:v" {
			assert.Equal(t, "1", args[i+1])
			break
		}
	}
}

func TestRecordingValidator_ExtractThumbnails_InvalidCount(
	t *testing.T,
) {
	v := NewRecordingValidator(logging.NullLogger{})
	_, err := v.ExtractThumbnails(
		context.Background(),
		"/tmp/video.mp4", "/tmp/thumbs", 0,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestRecordingValidator_ExtractThumbnails_NegativeCount(
	t *testing.T,
) {
	v := NewRecordingValidator(logging.NullLogger{})
	_, err := v.ExtractThumbnails(
		context.Background(),
		"/tmp/video.mp4", "/tmp/thumbs", -1,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}
