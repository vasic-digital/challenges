package userflow

import (
	"context"
	"fmt"
	"strings"

	"digital.vasic.challenges/pkg/assertion"
	"digital.vasic.challenges/pkg/i18n"
)

// tr is a thin helper that calls the package-level translator and
// falls back to the previous English literal if no consumer has
// wired a real translator (NoopTranslator returns the messageID
// verbatim — we detect that and use the fallback so end-user-
// visible text is preserved without a translator). Evaluator
// signatures here are pre-existing (do not accept ctx), so we use
// context.Background() — translation MUST be a non-blocking
// in-memory lookup per CONST-051(B) decoupling guarantee.
func tr(
	id string,
	data map[string]any,
	fallback string,
) string {
	out, err := i18n.Pkg().T(
		context.Background(), id, data,
	)
	if err != nil || out == "" || out == id {
		return fallback
	}
	return out
}

// RegisterEvaluators registers all 19 userflow assertion
// evaluators with the given engine.
func RegisterEvaluators(
	engine *assertion.DefaultEngine,
) error {
	evaluators := map[string]assertion.Evaluator{
		"build_succeeds":          evaluateBuildSucceeds,
		"all_tests_pass":          evaluateAllTestsPass,
		"lint_passes":             evaluateLintPasses,
		"app_launches":            evaluateAppLaunches,
		"app_stable":              evaluateAppStable,
		"status_code":             evaluateStatusCode,
		"response_contains":       evaluateResponseContains,
		"response_not_empty":      evaluateResponseNotEmpty,
		"json_field_equals":       evaluateJSONFieldEquals,
		"screenshot_exists":       evaluateScreenshotExists,
		"flow_completes":          evaluateFlowCompletes,
		"within_duration":         evaluateWithinDuration,
		"vision_element_detected": evaluateVisionElementDetected,
		"vision_confidence_above": evaluateVisionConfidenceAbove,
		"video_recorded":          evaluateVideoRecorded,
		"video_duration_within":   evaluateVideoDurationWithin,
		"video_integrity":         evaluateVideoIntegrity,
		"tests_generated":         evaluateTestsGenerated,
		"generated_test_coverage": evaluateGeneratedTestCoverage,
	}

	for name, eval := range evaluators {
		if err := engine.Register(name, eval); err != nil {
			return fmt.Errorf(
				"register evaluator %s: %w", name, err,
			)
		}
	}
	return nil
}

// toIntVal converts a value to int. Supports int, int64,
// float64, and float32.
func toIntVal(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	default:
		return 0, false
	}
}

// evaluateBuildSucceeds checks that the value is a bool true.
func evaluateBuildSucceeds(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.(bool)
	if !ok {
		return false, tr(
			"challenges_userflow_evaluators_build_succeeds_wrong_type",
			map[string]any{
				"gotType": fmt.Sprintf("%T", value),
			},
			fmt.Sprintf(
				"build_succeeds: expected bool, got %T", value,
			),
		)
	}
	if b {
		return true, "build succeeded"
	}
	return false, "build failed"
}

// evaluateAllTestsPass checks that the failure count is 0.
func evaluateAllTestsPass(
	def assertion.Definition, value any,
) (bool, string) {
	failures, ok := toIntVal(value)
	if !ok {
		return false, tr(
			"challenges_userflow_evaluators_all_tests_pass_wrong_type",
			map[string]any{
				"gotType": fmt.Sprintf("%T", value),
			},
			fmt.Sprintf(
				"all_tests_pass: expected int, got %T", value,
			),
		)
	}
	if failures == 0 {
		return true, "all tests passed (0 failures)"
	}
	return false, tr(
		"challenges_userflow_evaluators_all_tests_pass_failures",
		map[string]any{"failures": failures},
		fmt.Sprintf(
			"tests failed: %d failures", failures,
		),
	)
}

// evaluateLintPasses checks that the value is a bool true.
func evaluateLintPasses(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.(bool)
	if !ok {
		return false, tr(
			"challenges_userflow_evaluators_lint_passes_wrong_type",
			map[string]any{
				"gotType": fmt.Sprintf("%T", value),
			},
			fmt.Sprintf(
				"lint_passes: expected bool, got %T", value,
			),
		)
	}
	if b {
		return true, "lint passed"
	}
	return false, "lint failed"
}

// evaluateAppLaunches checks that the value is a bool true.
func evaluateAppLaunches(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.(bool)
	if !ok {
		return false, tr(
			"challenges_userflow_evaluators_app_launches_wrong_type",
			map[string]any{
				"gotType": fmt.Sprintf("%T", value),
			},
			fmt.Sprintf(
				"app_launches: expected bool, got %T", value,
			),
		)
	}
	if b {
		return true, "app launched successfully"
	}
	return false, "app failed to launch"
}

// evaluateAppStable checks that the value is a bool true.
func evaluateAppStable(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Sprintf(
			"app_stable: expected bool, got %T", value,
		)
	}
	if b {
		return true, "app is stable"
	}
	return false, "app is unstable"
}

// evaluateStatusCode checks that the int value equals
// def.Value.
func evaluateStatusCode(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toIntVal(value)
	if !ok {
		return false, fmt.Sprintf(
			"status_code: expected int, got %T", value,
		)
	}
	expected, ok := toIntVal(def.Value)
	if !ok {
		return false, fmt.Sprintf(
			"status_code: expected int for def.Value, got %T",
			def.Value,
		)
	}
	if actual == expected {
		return true, fmt.Sprintf(
			"status code is %d", actual,
		)
	}
	return false, tr(
		"challenges_userflow_evaluators_status_code_mismatch",
		map[string]any{
			"expected": expected,
			"actual":   actual,
		},
		fmt.Sprintf(
			"status code: expected %d, got %d",
			expected, actual,
		),
	)
}

// evaluateResponseContains checks that the string value
// contains def.Value.
func evaluateResponseContains(
	def assertion.Definition, value any,
) (bool, string) {
	s, ok := value.(string)
	if !ok {
		return false, fmt.Sprintf(
			"response_contains: expected string, got %T",
			value,
		)
	}
	expected, ok := def.Value.(string)
	if !ok {
		return false, fmt.Sprintf(
			"response_contains: expected string for "+
				"def.Value, got %T", def.Value,
		)
	}
	if strings.Contains(s, expected) {
		return true, fmt.Sprintf(
			"response contains %q", expected,
		)
	}
	return false, fmt.Sprintf(
		"response does not contain %q", expected,
	)
}

// evaluateResponseNotEmpty checks that the value has
// non-zero length. Supports string and []byte.
func evaluateResponseNotEmpty(
	def assertion.Definition, value any,
) (bool, string) {
	switch v := value.(type) {
	case string:
		if len(v) > 0 {
			return true, "response is not empty"
		}
		return false, "response is empty"
	case []byte:
		if len(v) > 0 {
			return true, "response is not empty"
		}
		return false, "response is empty"
	default:
		return false, fmt.Sprintf(
			"response_not_empty: expected string or "+
				"[]byte, got %T", value,
		)
	}
}

// evaluateJSONFieldEquals checks that the value equals
// def.Value using fmt.Sprintf comparison.
func evaluateJSONFieldEquals(
	def assertion.Definition, value any,
) (bool, string) {
	actual := fmt.Sprintf("%v", value)
	expected := fmt.Sprintf("%v", def.Value)
	if actual == expected {
		return true, fmt.Sprintf(
			"field equals %q", expected,
		)
	}
	return false, fmt.Sprintf(
		"field: expected %q, got %q", expected, actual,
	)
}

// evaluateScreenshotExists checks that the []byte value
// has non-zero length.
func evaluateScreenshotExists(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.([]byte)
	if !ok {
		return false, fmt.Sprintf(
			"screenshot_exists: expected []byte, got %T",
			value,
		)
	}
	if len(b) > 0 {
		return true, "screenshot captured"
	}
	return false, "screenshot is empty"
}

// evaluateFlowCompletes checks that the value is a bool
// true.
func evaluateFlowCompletes(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Sprintf(
			"flow_completes: expected bool, got %T", value,
		)
	}
	if b {
		return true, "flow completed successfully"
	}
	return false, "flow did not complete"
}

// evaluateWithinDuration checks that the int value (ms)
// is <= def.Value (ms).
func evaluateWithinDuration(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toIntVal(value)
	if !ok {
		return false, fmt.Sprintf(
			"within_duration: expected int, got %T", value,
		)
	}
	limit, ok := toIntVal(def.Value)
	if !ok {
		return false, fmt.Sprintf(
			"within_duration: expected int for def.Value, "+
				"got %T", def.Value,
		)
	}
	if actual <= limit {
		return true, fmt.Sprintf(
			"duration %dms within limit %dms",
			actual, limit,
		)
	}
	return false, fmt.Sprintf(
		"duration %dms exceeds limit %dms", actual, limit,
	)
}

// evaluateVisionElementDetected checks that the detected
// element count is >= def.Value.
func evaluateVisionElementDetected(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toIntVal(value)
	if !ok {
		return false, fmt.Sprintf(
			"vision_element_detected: expected int, got %T",
			value,
		)
	}
	expected, ok := toIntVal(def.Value)
	if !ok {
		expected = 1
	}
	if actual >= expected {
		return true, fmt.Sprintf(
			"detected %d elements (>= %d)", actual, expected,
		)
	}
	return false, fmt.Sprintf(
		"detected %d elements, expected >= %d",
		actual, expected,
	)
}

// evaluateVisionConfidenceAbove checks that the float64
// value is >= def.Value.
func evaluateVisionConfidenceAbove(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toFloat64Val(value)
	if !ok {
		return false, fmt.Sprintf(
			"vision_confidence_above: expected float, "+
				"got %T", value,
		)
	}
	expected, ok := toFloat64Val(def.Value)
	if !ok {
		expected = 0.5
	}
	if actual >= expected {
		return true, fmt.Sprintf(
			"confidence %.2f >= %.2f", actual, expected,
		)
	}
	return false, fmt.Sprintf(
		"confidence %.2f < %.2f", actual, expected,
	)
}

// evaluateVideoRecorded checks that the value is true.
func evaluateVideoRecorded(
	def assertion.Definition, value any,
) (bool, string) {
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Sprintf(
			"video_recorded: expected bool, got %T", value,
		)
	}
	if b {
		return true, "video was recorded"
	}
	return false, "video was not recorded"
}

// evaluateVideoDurationWithin checks that the duration (ms)
// is <= def.Value (ms).
func evaluateVideoDurationWithin(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toIntVal(value)
	if !ok {
		return false, fmt.Sprintf(
			"video_duration_within: expected int, got %T",
			value,
		)
	}
	limit, ok := toIntVal(def.Value)
	if !ok {
		return false, fmt.Sprintf(
			"video_duration_within: expected int for "+
				"def.Value, got %T", def.Value,
		)
	}
	if actual <= limit {
		return true, fmt.Sprintf(
			"video duration %dms within %dms",
			actual, limit,
		)
	}
	return false, fmt.Sprintf(
		"video duration %dms exceeds %dms", actual, limit,
	)
}

// evaluateVideoIntegrity checks that the RecordingResult
// has non-zero file size, duration, and frame count. The
// value should be a map with keys: file_size, duration_ms,
// frame_count.
func evaluateVideoIntegrity(
	def assertion.Definition, value any,
) (bool, string) {
	m, ok := value.(map[string]any)
	if !ok {
		return false, fmt.Sprintf(
			"video_integrity: expected map[string]any, "+
				"got %T", value,
		)
	}
	fileSize, _ := toIntVal(m["file_size"])
	durationMs, _ := toIntVal(m["duration_ms"])
	frameCount, _ := toIntVal(m["frame_count"])

	if fileSize <= 0 {
		return false, tr(
			"challenges_userflow_evaluators_video_integrity_zero_filesize",
			map[string]any{"fileSize": fileSize},
			fmt.Sprintf(
				"video integrity: file_size is %d (must be > 0)",
				fileSize,
			),
		)
	}
	if durationMs <= 0 {
		return false, tr(
			"challenges_userflow_evaluators_video_integrity_zero_duration",
			map[string]any{"durationMs": durationMs},
			fmt.Sprintf(
				"video integrity: duration is %dms "+
					"(must be > 0)", durationMs,
			),
		)
	}
	if frameCount <= 0 {
		return false, tr(
			"challenges_userflow_evaluators_video_integrity_zero_frames",
			map[string]any{"frameCount": frameCount},
			fmt.Sprintf(
				"video integrity: frame_count is %d "+
					"(must be > 0)", frameCount,
			),
		)
	}
	return true, tr(
		"challenges_userflow_evaluators_video_integrity_summary",
		map[string]any{
			"fileSize":   fileSize,
			"durationMs": durationMs,
			"frameCount": frameCount,
		},
		fmt.Sprintf(
			"video integrity: %d bytes, %dms, %d frames",
			fileSize, durationMs, frameCount,
		),
	)
}

// evaluateTestsGenerated checks that the generated test
// count is >= def.Value.
func evaluateTestsGenerated(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toIntVal(value)
	if !ok {
		return false, fmt.Sprintf(
			"tests_generated: expected int, got %T", value,
		)
	}
	expected, ok := toIntVal(def.Value)
	if !ok {
		expected = 1
	}
	if actual >= expected {
		return true, fmt.Sprintf(
			"generated %d tests (>= %d)", actual, expected,
		)
	}
	return false, fmt.Sprintf(
		"generated %d tests, expected >= %d",
		actual, expected,
	)
}

// evaluateGeneratedTestCoverage checks that the category
// count is >= def.Value.
func evaluateGeneratedTestCoverage(
	def assertion.Definition, value any,
) (bool, string) {
	actual, ok := toIntVal(value)
	if !ok {
		return false, fmt.Sprintf(
			"generated_test_coverage: expected int, got %T",
			value,
		)
	}
	expected, ok := toIntVal(def.Value)
	if !ok {
		expected = 1
	}
	if actual >= expected {
		return true, fmt.Sprintf(
			"covers %d categories (>= %d)",
			actual, expected,
		)
	}
	return false, fmt.Sprintf(
		"covers %d categories, expected >= %d",
		actual, expected,
	)
}

// toFloat64Val converts a value to float64.
func toFloat64Val(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
