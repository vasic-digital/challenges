package userflow

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"digital.vasic.challenges/pkg/assertion"
	"digital.vasic.challenges/pkg/i18n"
)

// CONST-050(A): mocks permitted in unit tests only.
//
// These tests assert that the 10 round-100 CONST-046 migrated call
// sites in evaluators.go ROUTE through the package-level translator
// rather than emit their previous hardcoded English literals.
//
// For each migrated call site we (a) install a sentinel translator
// returning "<TRANSLATED:msgID>", (b) trigger the evaluator code
// path, (c) assert the returned message string contains the
// sentinel — NOT the original English literal. The presence of the
// sentinel is positive evidence per CONST-035 / §11.4.2 that the
// translation seam is exercised at runtime.

// sentinelTranslator emits a deterministic marker so tests can
// detect that the translator was invoked.
type sentinelTranslator struct{}

func (sentinelTranslator) T(
	_ context.Context,
	id string,
	_ map[string]any,
) (string, error) {
	return fmt.Sprintf("<TRANSLATED:%s>", id), nil
}

func (sentinelTranslator) TPlural(
	_ context.Context,
	id string,
	_ int,
	_ map[string]any,
) (string, error) {
	return fmt.Sprintf("<TRANSLATED:%s>", id), nil
}

func withSentinelTranslator(t *testing.T) {
	t.Helper()
	i18n.SetPkgTranslator(sentinelTranslator{})
	t.Cleanup(func() { i18n.SetPkgTranslator(nil) })
}

// evaluator-call helpers: each invokes the production evaluator
// and returns the human-readable message string.

func callBuildSucceeds(value any) string {
	_, msg := evaluateBuildSucceeds(
		assertion.Definition{}, value,
	)
	return msg
}

func callAllTestsPass(value any) string {
	_, msg := evaluateAllTestsPass(
		assertion.Definition{}, value,
	)
	return msg
}

func callLintPasses(value any) string {
	_, msg := evaluateLintPasses(
		assertion.Definition{}, value,
	)
	return msg
}

func callAppLaunches(value any) string {
	_, msg := evaluateAppLaunches(
		assertion.Definition{}, value,
	)
	return msg
}

func callStatusCode(expected, actual int) string {
	_, msg := evaluateStatusCode(
		assertion.Definition{Value: expected}, actual,
	)
	return msg
}

func callVideoIntegrity(m map[string]any) string {
	_, msg := evaluateVideoIntegrity(
		assertion.Definition{}, m,
	)
	return msg
}

func TestRound100_MigratedSitesRouteThroughTranslator(t *testing.T) {
	withSentinelTranslator(t)

	cases := []struct {
		name     string
		call     func() string
		sentinel string
	}{
		{
			name:     "build_succeeds_wrong_type",
			call:     func() string { return callBuildSucceeds("not a bool") },
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_build_succeeds_wrong_type>",
		},
		{
			name:     "all_tests_pass_wrong_type",
			call:     func() string { return callAllTestsPass("not int") },
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_all_tests_pass_wrong_type>",
		},
		{
			name:     "all_tests_pass_failures",
			call:     func() string { return callAllTestsPass(3) },
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_all_tests_pass_failures>",
		},
		{
			name:     "lint_passes_wrong_type",
			call:     func() string { return callLintPasses(42) },
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_lint_passes_wrong_type>",
		},
		{
			name:     "app_launches_wrong_type",
			call:     func() string { return callAppLaunches("nope") },
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_app_launches_wrong_type>",
		},
		{
			name:     "status_code_mismatch",
			call:     func() string { return callStatusCode(200, 500) },
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_status_code_mismatch>",
		},
		{
			name: "video_integrity_zero_filesize",
			call: func() string {
				return callVideoIntegrity(map[string]any{
					"file_size":   0,
					"duration_ms": 100,
					"frame_count": 10,
				})
			},
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_video_integrity_zero_filesize>",
		},
		{
			name: "video_integrity_zero_duration",
			call: func() string {
				return callVideoIntegrity(map[string]any{
					"file_size":   100,
					"duration_ms": 0,
					"frame_count": 10,
				})
			},
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_video_integrity_zero_duration>",
		},
		{
			name: "video_integrity_zero_frames",
			call: func() string {
				return callVideoIntegrity(map[string]any{
					"file_size":   100,
					"duration_ms": 100,
					"frame_count": 0,
				})
			},
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_video_integrity_zero_frames>",
		},
		{
			name: "video_integrity_summary",
			call: func() string {
				return callVideoIntegrity(map[string]any{
					"file_size":   100,
					"duration_ms": 200,
					"frame_count": 30,
				})
			},
			sentinel: "<TRANSLATED:challenges_userflow_evaluators_video_integrity_summary>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.call()
			if !strings.Contains(msg, tc.sentinel) {
				t.Fatalf(
					"%s: msg=%q does not contain sentinel %q "+
						"(translator seam not exercised)",
					tc.name, msg, tc.sentinel,
				)
			}
			// Anti-bluff: also assert NO leakage of the bare
			// English literal (would prove the migration is real,
			// not a wrapper that still emits original literal).
			if strings.Contains(msg, "expected bool, got string") &&
				tc.name == "build_succeeds_wrong_type" {
				t.Fatalf(
					"%s: original literal leaked into output",
					tc.name,
				)
			}
		})
	}
}

func TestRound100_FallbackPreservesOriginalEnglish(t *testing.T) {
	// With default NoopTranslator, the fallback (original English)
	// MUST be returned so end-user-visible text is preserved when
	// no consumer has wired a translator.
	i18n.SetPkgTranslator(nil) // explicit reset
	msg := callBuildSucceeds("not a bool")
	if !strings.Contains(msg, "build_succeeds: expected bool, got") {
		t.Fatalf(
			"fallback English literal missing under "+
				"NoopTranslator: msg=%q", msg,
		)
	}
}
