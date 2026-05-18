package userflow

import (
	"errors"
	"strings"
	"testing"

	"digital.vasic.challenges/pkg/i18n"
)

// CONST-050(A): mocks permitted in unit tests only.
//
// These tests assert that the 10 round-102 CONST-046 migrated call
// sites in challenge_desktop.go ROUTE through the package-level
// translator rather than emit their previous hardcoded English
// literals.
//
// For each migrated call site we (a) install a sentinel translator
// (reusing the helper defined in evaluators_i18n_test.go) returning
// "<TRANSLATED:msgID>", (b) trigger the helper code path,
// (c) assert the returned message string contains the sentinel —
// NOT the original English literal. The presence of the sentinel is
// positive evidence per CONST-035 / §11.4.2 that the translation
// seam is exercised at runtime.

func TestRound102_DesktopMigratedSitesRouteThroughTranslator(t *testing.T) {
	withSentinelTranslator(t)

	bootErr := errors.New("boot timeout")

	cases := []struct {
		name     string
		call     func() string
		sentinel string
	}{
		{
			name:     "desktop_launched_successfully",
			call:     func() string { return desktopLaunchMessage(nil) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_launched_successfully>",
		},
		{
			name:     "desktop_launch_failed",
			call:     func() string { return desktopLaunchMessage(bootErr) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_launch_failed>",
		},
		{
			name:     "desktop_window_appeared",
			call:     func() string { return windowMessage(nil) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_window_appeared>",
		},
		{
			name:     "desktop_window_failed",
			call:     func() string { return windowMessage(bootErr) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_window_failed>",
		},
		{
			name:     "desktop_stability_failed",
			call:     func() string { return desktopStabilityMessage(false, bootErr) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_stability_failed>",
		},
		{
			name:     "desktop_stable",
			call:     func() string { return desktopStabilityMessage(true, nil) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_stable>",
		},
		{
			name:     "desktop_crashed",
			call:     func() string { return desktopStabilityMessage(false, nil) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_crashed>",
		},
		{
			name:     "desktop_ipc_succeeded",
			call:     func() string { return ipcMessage("ping", true, nil) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_ipc_succeeded>",
		},
		{
			name:     "desktop_ipc_failed",
			call:     func() string { return ipcMessage("ping", false, bootErr) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_ipc_failed>",
		},
		{
			name:     "desktop_ipc_mismatch",
			call:     func() string { return ipcMessage("ping", false, nil) },
			sentinel: "<TRANSLATED:challenges_userflow_desktop_ipc_mismatch>",
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
			// Anti-bluff: a stray full English literal in output
			// would prove the migration is a wrapper bluff. Spot-
			// check a high-signal phrase per call-site family.
			if tc.name == "desktop_launched_successfully" &&
				strings.Contains(msg, "desktop app launched successfully") {
				t.Fatalf(
					"%s: original literal leaked into output",
					tc.name,
				)
			}
			if tc.name == "desktop_ipc_succeeded" &&
				strings.Contains(msg, "succeeded") &&
				!strings.Contains(msg, "<TRANSLATED:") {
				t.Fatalf(
					"%s: original literal leaked into output",
					tc.name,
				)
			}
		})
	}
}

func TestRound102_DesktopFallbackPreservesOriginalEnglish(t *testing.T) {
	// With default NoopTranslator (and no consumer-side wiring),
	// the fallback English MUST be returned so end-user-visible
	// text is preserved when no consumer has installed a real
	// Translator.
	i18n.SetPkgTranslator(nil) // explicit reset to NoopTranslator
	t.Cleanup(func() { i18n.SetPkgTranslator(nil) })
	msg := desktopLaunchMessage(nil)
	if !strings.Contains(msg, "desktop app launched successfully") {
		t.Fatalf(
			"fallback English literal missing under "+
				"NoopTranslator: msg=%q", msg,
		)
	}
	msg2 := ipcMessage("ping", true, nil)
	if !strings.Contains(msg2, "IPC command") ||
		!strings.Contains(msg2, "succeeded") {
		t.Fatalf(
			"fallback English literal missing under "+
				"NoopTranslator: msg=%q", msg2,
		)
	}
}
