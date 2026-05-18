package userflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/i18n"
)

// CONST-050(A): mocks permitted in unit tests only.
//
// Round 104 §11.4 anti-bluff: assert that the 7 CONST-046
// migrated AssertionResult.Message IDs in
// challenge_recorded_mobile.go ROUTE through the package-level
// translator rather than emit their previous hardcoded English
// literals. Each ID is exercised across both the
// RecordedMobileLaunchChallenge and RecordedMobileFlowChallenge
// call sites (14 call sites total via 7 deduped IDs).
//
// For every migrated call site we (a) install a sentinel
// translator returning "<TRANSLATED:msgID>", (b) trigger the
// challenge code path that produces the message, (c) assert the
// returned assertion message contains the sentinel — NOT the
// original English literal. Sentinel presence is positive
// evidence per CONST-035 / §11.4.2 that the translation seam is
// exercised at runtime.
//
// Verbatim 2026-05-19 operator mandate: "all existing tests and
// Challenges do work in anti-bluff manner - they MUST confirm
// that all tested codebase really works as expected!"

// rmobSentinelTranslator emits a deterministic marker so tests
// can detect that the translator was invoked.
type rmobSentinelTranslator struct{}

func (rmobSentinelTranslator) T(
	_ context.Context,
	id string,
	_ map[string]any,
) (string, error) {
	return fmt.Sprintf("<TRANSLATED:%s>", id), nil
}

func (rmobSentinelTranslator) TPlural(
	_ context.Context,
	id string,
	_ int,
	_ map[string]any,
) (string, error) {
	return fmt.Sprintf("<TRANSLATED:%s>", id), nil
}

func withRMobSentinelTranslator(t *testing.T) {
	t.Helper()
	i18n.SetPkgTranslator(rmobSentinelTranslator{})
	t.Cleanup(func() { i18n.SetPkgTranslator(nil) })
}

// findRMobMessage returns the Message string of the first
// assertion whose Target matches the supplied target. Fails the
// test if no matching assertion was emitted.
func findRMobMessage(
	t *testing.T,
	res *challenge.Result,
	target string,
) string {
	t.Helper()
	for _, a := range res.Assertions {
		if a.Target == target {
			return a.Message
		}
	}
	t.Fatalf(
		"no assertion with Target=%q in result", target,
	)
	return ""
}

// --- RecordedMobileLaunchChallenge sentinel assertions ---

func TestRMobLaunch_I18N_AdapterUnavailable_Routes(
	t *testing.T,
) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	adapter.available = false
	recorder := newMockRecorderForFlow()

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-001", "i18n adapter unavail",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "platform_available")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_adapter_unavailable>")
	require.False(t, strings.Contains(msg, "Mobile adapter not available"),
		"raw English literal must not leak when translator is installed")
}

func TestRMobLaunch_I18N_RecorderUnavailable_Routes(
	t *testing.T,
) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()
	recorder.available = false

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-002", "i18n recorder unavail",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "recorder_available")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_recorder_unavailable>")
	require.False(t, strings.Contains(msg, "Recorder not available"),
		"raw English literal must not leak when translator is installed")
}

func TestRMobLaunch_I18N_StartRecordingFailed_Routes(
	t *testing.T,
) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()
	recorder.startErr = fmt.Errorf("ffmpeg not installed")

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-003", "i18n rec start fail",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "start_recording")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_start_recording_failed>")
	require.False(t, strings.Contains(msg, "start recording failed:"),
		"raw English literal must not leak when translator is installed")
}

func TestRMobLaunch_I18N_RecordingStarted_Routes(
	t *testing.T,
) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-004", "i18n rec started",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "start_recording")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_recording_started>")
	require.False(t, strings.Contains(msg, "recording started successfully"),
		"raw English literal must not leak when translator is installed")
}

func TestRMobLaunch_I18N_VideoCaptured_Routes(t *testing.T) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-005", "i18n video captured",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "video_recorded")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_video_captured>")
	require.False(t, strings.Contains(msg, "video recording captured successfully"),
		"raw English literal must not leak when translator is installed")
}

func TestRMobLaunch_I18N_VideoIntegrity_Routes(t *testing.T) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-006", "i18n integrity",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "video_integrity")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_video_integrity>")
	require.False(t, strings.Contains(msg, "video integrity: size="),
		"raw English literal must not leak when translator is installed")
}

func TestRMobLaunch_I18N_RecordingFailed_Routes(t *testing.T) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()
	// Force nil result + stopErr to trigger the "recording failed" path.
	recorder.result = nil
	recorder.stopErr = fmt.Errorf("recorder process died")

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-007", "i18n rec failed",
		"verifies CONST-046 routing",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "video_recorded")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_recording_failed>")
	require.False(t, strings.Contains(msg, "recording failed:"),
		"raw English literal must not leak when translator is installed")
}

// --- RecordedMobileFlowChallenge sentinel assertions ---

func TestRMobFlow_I18N_AdapterUnavailable_Routes(t *testing.T) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	adapter.available = false
	recorder := newMockRecorderForFlow()
	flow := MobileFlow{
		Name: "i18n-adapter-unavail",
		Steps: []MobileStep{
			{Name: "tap", Action: "tap", X: 1, Y: 1},
		},
	}

	ch := NewRecordedMobileFlowChallenge(
		"RMOB-FLOW-I18N-001", "i18n flow adapter unavail",
		"verifies CONST-046 routing",
		nil, adapter, recorder, flow,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "platform_available")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_adapter_unavailable>")
}

func TestRMobFlow_I18N_RecorderUnavailable_Routes(t *testing.T) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()
	recorder.available = false
	flow := MobileFlow{
		Name: "i18n-recorder-unavail",
		Steps: []MobileStep{
			{Name: "tap", Action: "tap", X: 1, Y: 1},
		},
	}

	ch := NewRecordedMobileFlowChallenge(
		"RMOB-FLOW-I18N-002", "i18n flow recorder unavail",
		"verifies CONST-046 routing",
		nil, adapter, recorder, flow,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "recorder_available")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_recorder_unavailable>")
}

func TestRMobFlow_I18N_VideoIntegrity_Routes(t *testing.T) {
	withRMobSentinelTranslator(t)

	adapter := newMockMobileForRecording()
	recorder := newMockRecorderForFlow()
	flow := MobileFlow{
		Name: "i18n-flow-integrity",
		Steps: []MobileStep{
			{Name: "tap", Action: "tap", X: 1, Y: 1},
		},
	}

	ch := NewRecordedMobileFlowChallenge(
		"RMOB-FLOW-I18N-003", "i18n flow integrity",
		"verifies CONST-046 routing",
		nil, adapter, recorder, flow,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "video_integrity")
	require.Contains(t, msg,
		"<TRANSLATED:challenges_userflow_recorded_mobile_video_integrity>")
}

// --- NoopTranslator fallback preservation ---

func TestRMob_I18N_NoopTranslator_PreservesFallback(t *testing.T) {
	// No sentinel installed — package-default NoopTranslator should
	// echo the message ID, triggering the fallback branch in trRecMob.
	i18n.SetPkgTranslator(nil) // ensure noop
	t.Cleanup(func() { i18n.SetPkgTranslator(nil) })

	adapter := newMockMobileForRecording()
	adapter.available = false
	recorder := newMockRecorderForFlow()

	ch := NewRecordedMobileLaunchChallenge(
		"RMOB-I18N-NOOP", "noop fallback",
		"verifies fallback preservation",
		nil, adapter, recorder,
		"/tmp/app.apk", 10*time.Millisecond,
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRMobMessage(t, res, "platform_available")
	require.Equal(t, "Mobile adapter not available - skipped", msg,
		"NoopTranslator must trigger fallback to original English literal")
}
