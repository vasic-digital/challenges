package panoptic

import (
	"encoding/json"
	"fmt"
	"os"

	"digital.vasic.challenges/pkg/challenge"
)

// ParseResultToAssertionValues converts a PanopticRunResult into
// a map suitable for assertion evaluation. The keys match the
// evaluator target names defined in evaluators.go.
func ParseResultToAssertionValues(
	r *PanopticRunResult,
) map[string]any {
	if r == nil {
		return map[string]any{}
	}

	values := map[string]any{
		"exit_code":         r.ExitCode,
		"all_apps_passed":   allAppsPassed(r),
		"app_count":         len(r.Apps),
		"passed_count":      countPassed(r),
		"failed_count":      countFailed(r),
		"total_screenshots": len(r.Screenshots),
		"total_videos":      len(r.Videos),
		"total_duration_ms": r.Duration.Milliseconds(),
		"max_duration_ms":   maxAppDuration(r),
		"report_html_exists": r.ReportHTML != "" &&
			fileExists(r.ReportHTML),
		"report_json_exists": r.ReportJSON != "" &&
			fileExists(r.ReportJSON),
		"screenshots": toAnySlice(r.Screenshots),
		"videos":      toAnySlice(r.Videos),
		"stdout":      r.Stdout,
		"stderr":      r.Stderr,
	}

	// Always include AI report keys so assertion evaluators
	// can be called (they handle empty strings gracefully).
	values["ai_error_report"] = r.AIErrorReport
	values["ai_generated_tests"] = r.AIGeneratedTests
	values["vision_report"] = r.VisionReport

	// Add per-app AI confidence if available.
	if confidence := extractAIConfidence(r); confidence >= 0 {
		values["ai_confidence"] = confidence
	}

	return values
}

// ParseResultToMetrics converts a PanopticRunResult into a map
// of MetricValue entries for inclusion in challenge results.
func ParseResultToMetrics(
	r *PanopticRunResult,
) map[string]challenge.MetricValue {
	if r == nil {
		return map[string]challenge.MetricValue{}
	}

	metrics := map[string]challenge.MetricValue{
		"total_duration_ms": {
			Name:  "total_duration_ms",
			Value: float64(r.Duration.Milliseconds()),
			Unit:  "ms",
		},
		"app_count": {
			Name:  "app_count",
			Value: float64(len(r.Apps)),
			Unit:  "count",
		},
		"screenshot_count": {
			Name:  "screenshot_count",
			Value: float64(len(r.Screenshots)),
			Unit:  "count",
		},
		"video_count": {
			Name:  "video_count",
			Value: float64(len(r.Videos)),
			Unit:  "count",
		},
		"passed_count": {
			Name:  "passed_count",
			Value: float64(countPassed(r)),
			Unit:  "count",
		},
		"failed_count": {
			Name:  "failed_count",
			Value: float64(countFailed(r)),
			Unit:  "count",
		},
	}

	if maxDur := maxAppDuration(r); maxDur > 0 {
		metrics["max_app_duration_ms"] = challenge.MetricValue{
			Name:  "max_app_duration_ms",
			Value: float64(maxDur),
			Unit:  "ms",
		}
	}

	for i, app := range r.Apps {
		key := fmt.Sprintf("app_%d_duration_ms", i)
		metrics[key] = challenge.MetricValue{
			Name:  key,
			Value: float64(app.DurationMs),
			Unit:  "ms",
		}
	}

	return metrics
}

// --- helpers ---

func allAppsPassed(r *PanopticRunResult) bool {
	if len(r.Apps) == 0 {
		return r.ExitCode == 0
	}
	for _, app := range r.Apps {
		if !app.Success {
			return false
		}
	}
	return true
}

func countPassed(r *PanopticRunResult) int {
	count := 0
	for _, app := range r.Apps {
		if app.Success {
			count++
		}
	}
	return count
}

func countFailed(r *PanopticRunResult) int {
	count := 0
	for _, app := range r.Apps {
		if !app.Success {
			count++
		}
	}
	return count
}

func maxAppDuration(r *PanopticRunResult) int64 {
	var max int64
	for _, app := range r.Apps {
		if app.DurationMs > max {
			max = app.DurationMs
		}
	}
	return max
}

func toAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// extractAIConfidence reads the AI error report and extracts a
// confidence score. Returns -1 when:
//   - no AIErrorReport path is configured
//   - the file does not exist
//   - the file cannot be read or parsed
//   - no confidence field is present in the JSON
//
// Previously this function returned 1.0 (exit 0) / 0.5 (non-zero exit)
// based only on exit code without opening the JSON file — a §11.4
// PASS-bluff: any Challenge assertion on AI confidence was
// certifying a fabricated value derived from the exit code, not the
// real model output. Now it parses the JSON and reads the actual
// confidence field (top-level "confidence" or "ai_confidence"); on
// any parse failure it returns -1 so the caller's "if confidence
// >= 0" gate correctly omits the value rather than asserting on a
// lie.
func extractAIConfidence(r *PanopticRunResult) float64 {
	if r.AIErrorReport == "" {
		return -1
	}
	if !fileExists(r.AIErrorReport) {
		return -1
	}
	data, err := os.ReadFile(r.AIErrorReport)
	if err != nil {
		return -1
	}
	// Accept either flat {"confidence": ...} or {"ai_confidence": ...}
	// as the canonical fields. Other layouts return -1 (unknown).
	var payload struct {
		Confidence   *float64 `json:"confidence"`
		AIConfidence *float64 `json:"ai_confidence"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return -1
	}
	switch {
	case payload.Confidence != nil:
		return *payload.Confidence
	case payload.AIConfidence != nil:
		return *payload.AIConfidence
	default:
		return -1
	}
}
