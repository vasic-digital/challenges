// SPDX-FileCopyrightText: 2025-2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package userflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"digital.vasic.challenges/pkg/logging"
)

// ValidationResult contains the outcome of a recording
// validation check.
type ValidationResult struct {
	FileSize       int64    `json:"file_size"`
	Duration       float64  `json:"duration"`
	FrameCount     int      `json:"frame_count"`
	HasBlackFrames bool     `json:"has_black_frames"`
	IsValid        bool     `json:"is_valid"`
	Errors         []string `json:"errors"`
}

// RecordingValidator checks recorded video files for quality:
// file existence, duration, black frame detection, and
// thumbnail extraction. Uses ffprobe and ffmpeg for analysis.
type RecordingValidator struct {
	logger logging.Logger
}

// NewRecordingValidator creates a RecordingValidator with the
// given logger.
func NewRecordingValidator(
	logger logging.Logger,
) *RecordingValidator {
	return &RecordingValidator{logger: logger}
}

// Validate performs a full quality check on the given video
// file: verifies it exists, has non-zero size, positive
// duration, and checks for all-black frames.
func (v *RecordingValidator) Validate(
	ctx context.Context, filePath string,
) (*ValidationResult, error) {
	result := &ValidationResult{
		Errors: make([]string, 0),
	}

	// Check file exists and has content.
	info, err := os.Stat(filePath)
	if err != nil {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("file not found: %s", filePath),
		)
		return result, nil
	}
	result.FileSize = info.Size()
	if result.FileSize == 0 {
		result.Errors = append(
			result.Errors, "file is empty (0 bytes)",
		)
		return result, nil
	}

	// Get duration via ffprobe.
	duration, err := v.probeDuration(ctx, filePath)
	if err != nil {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("duration probe failed: %v", err),
		)
	} else {
		result.Duration = duration
		if duration <= 0 {
			result.Errors = append(
				result.Errors,
				"duration is zero or negative",
			)
		}
	}

	// Get frame count via ffprobe.
	frameCount, err := v.probeFrameCount(ctx, filePath)
	if err != nil {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("frame count probe failed: %v", err),
		)
	} else {
		result.FrameCount = frameCount
	}

	// Check for black frames.
	hasBlack, err := v.detectBlackFrames(ctx, filePath)
	if err != nil {
		result.Errors = append(
			result.Errors,
			fmt.Sprintf(
				"black frame detection failed: %v", err,
			),
		)
	}
	result.HasBlackFrames = hasBlack

	// Determine overall validity.
	result.IsValid = len(result.Errors) == 0 &&
		result.FileSize > 0 &&
		result.Duration > 0 &&
		!result.HasBlackFrames

	return result, nil
}

// ExtractThumbnails extracts key frame thumbnails from the
// video file. Returns a slice of file paths for the
// extracted thumbnails.
func (v *RecordingValidator) ExtractThumbnails(
	ctx context.Context,
	filePath, outputDir string,
	count int,
) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf(
			"thumbnail count must be positive",
		)
	}

	// Create output directory if needed.
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf(
			"create output dir: %w", err,
		)
	}

	args := buildThumbnailArgs(
		filePath, outputDir, count,
	)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"extract thumbnails: %w", err,
		)
	}

	// Collect generated file paths.
	paths := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		path := filepath.Join(
			outputDir,
			fmt.Sprintf("thumb_%04d.png", i),
		)
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}

	v.logger.Info("thumbnails extracted",
		logging.IntField("count", len(paths)),
		logging.StringField("dir", outputDir),
	)
	return paths, nil
}

// Available returns true if both ffprobe and ffmpeg are
// installed and can be found in PATH.
func (v *RecordingValidator) Available(
	ctx context.Context,
) bool {
	_, err1 := exec.LookPath("ffprobe")
	_, err2 := exec.LookPath("ffmpeg")
	return err1 == nil && err2 == nil
}

// probeDuration returns the video duration in seconds using
// ffprobe.
func (v *RecordingValidator) probeDuration(
	ctx context.Context, filePath string,
) (float64, error) {
	args := buildDurationProbeArgs(filePath)
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}

	durStr := strings.TrimSpace(string(out))
	duration, err := strconv.ParseFloat(durStr, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"parse duration %q: %w", durStr, err,
		)
	}
	return duration, nil
}

// probeFrameCount returns the number of video frames using
// ffprobe.
func (v *RecordingValidator) probeFrameCount(
	ctx context.Context, filePath string,
) (int, error) {
	args := buildFrameCountProbeArgs(filePath)
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf(
			"ffprobe frame count: %w", err,
		)
	}

	countStr := strings.TrimSpace(string(out))
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0, fmt.Errorf(
			"parse frame count %q: %w", countStr, err,
		)
	}
	return count, nil
}

// detectBlackFrames checks if the video has significant
// black frames using ffmpeg's blackdetect filter.
func (v *RecordingValidator) detectBlackFrames(
	ctx context.Context, filePath string,
) (bool, error) {
	args := buildBlackDetectArgs(filePath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// blackdetect outputs to stderr.
	out, err := cmd.CombinedOutput()
	if err != nil {
		// ffmpeg returns non-zero for -f null, but
		// blackdetect output is still valid on stderr.
		// Only fail if there is no output at all.
		if len(out) == 0 {
			return false, fmt.Errorf(
				"black detect: %w", err,
			)
		}
	}

	// If "black_start" appears in the output, black frames
	// were detected.
	return strings.Contains(
		string(out), "black_start",
	), nil
}

// buildDurationProbeArgs constructs the ffprobe argument
// list for extracting video duration.
func buildDurationProbeArgs(filePath string) []string {
	return []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		filePath,
	}
}

// buildFrameCountProbeArgs constructs the ffprobe argument
// list for extracting the video frame count.
func buildFrameCountProbeArgs(filePath string) []string {
	return []string{
		"-v", "error",
		"-count_frames",
		"-select_streams", "v:0",
		"-show_entries", "stream=nb_read_frames",
		"-of", "csv=p=0",
		filePath,
	}
}

// buildBlackDetectArgs constructs the ffmpeg argument list
// for black frame detection.
func buildBlackDetectArgs(filePath string) []string {
	return []string{
		"-i", filePath,
		"-vf", "blackdetect=d=0.5:pix_th=0.1",
		"-an",
		"-f", "null",
		"-",
	}
}

// buildThumbnailArgs constructs the ffmpeg argument list
// for extracting evenly-spaced thumbnails.
func buildThumbnailArgs(
	filePath, outputDir string, count int,
) []string {
	pattern := filepath.Join(outputDir, "thumb_%04d.png")
	return []string{
		"-i", filePath,
		"-vf", fmt.Sprintf(
			"select=not(mod(n\\,%d))", max(count, 1),
		),
		"-vsync", "vfr",
		"-frames:v", strconv.Itoa(count),
		"-y",
		pattern,
	}
}
