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
// Round 101 §11.4 anti-bluff: assert that the 10 CONST-046
// migrated AssertionResult.Message strings in
// challenge_recorded_ai_testgen.go ROUTE through the package-level
// translator rather than emit their previous hardcoded English
// literals.
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

// raitgSentinelTranslator emits a deterministic marker so tests
// can detect that the translator was invoked.
type raitgSentinelTranslator struct{}

func (raitgSentinelTranslator) T(
	_ context.Context,
	id string,
	_ map[string]any,
) (string, error) {
	return fmt.Sprintf("<TRANSLATED:%s>", id), nil
}

func (raitgSentinelTranslator) TPlural(
	_ context.Context,
	id string,
	_ int,
	_ map[string]any,
) (string, error) {
	return fmt.Sprintf("<TRANSLATED:%s>", id), nil
}

func withRAITGSentinelTranslator(t *testing.T) {
	t.Helper()
	i18n.SetPkgTranslator(raitgSentinelTranslator{})
	t.Cleanup(func() { i18n.SetPkgTranslator(nil) })
}

// findRAITGMessage returns the Message string of the first
// assertion whose Target matches the supplied target. Fails the
// test if no matching assertion was emitted.
func findRAITGMessage(
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

// buildRAITGChallenge constructs a challenge wired with the
// supplied mocks. Mocks reuse the existing
// newMockBrowserForRecording / newMockRecorderForFlow /
// mockTestGenForRecording helpers from the userflow package's
// existing unit-test file.
func buildRAITGChallenge(
	browser BrowserAdapter,
	recorder RecorderAdapter,
	testgen TestGenAdapter,
	url, outDir string,
) *RecordedAITestGenChallenge {
	return NewRecordedAITestGenChallenge(
		"RAITG-I18N", "RAITG i18n smoke",
		"verifies CONST-046 routing",
		nil, browser, recorder, testgen,
		url, 3, outDir,
	)
}

func TestRAITG_I18N_BrowserUnavailable_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	browser.available = false
	recorder := newMockRecorderForFlow()
	testgen := &mockTestGenForRecording{available: true}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "browser_available")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_browser_unavailable>",
	)
	require.False(t, strings.Contains(msg, "skipped"),
		"raw English literal must not leak when translator is installed")
}

func TestRAITG_I18N_RecorderUnavailable_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	recorder := newMockRecorderForFlow()
	recorder.available = false
	testgen := &mockTestGenForRecording{available: true}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "recorder_available")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_recorder_unavailable>",
	)
}

func TestRAITG_I18N_TestGenUnavailable_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	recorder := newMockRecorderForFlow()
	testgen := &mockTestGenForRecording{available: false}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "testgen_available")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_testgen_unavailable>",
	)
}

func TestRAITG_I18N_BrowserInitFailed_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	browser.initErr = fmt.Errorf("init boom")
	recorder := newMockRecorderForFlow()
	testgen := &mockTestGenForRecording{available: true}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "initialize")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_browser_init_failed>",
	)
}

func TestRAITG_I18N_StartRecordingFailed_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	recorder := newMockRecorderForFlow()
	recorder.startErr = fmt.Errorf("ffmpeg not installed")
	testgen := &mockTestGenForRecording{available: true}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "start_recording")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_start_recording_failed>",
	)
}

func TestRAITG_I18N_NavigateFailed_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	browser.navigateErr = fmt.Errorf("dns refused")
	recorder := newMockRecorderForFlow()
	testgen := &mockTestGenForRecording{available: true}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "target_url")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_navigate_failed>",
	)
}

// raitgFailScreenshotBrowser wraps mockBrowserForRecording but
// overrides Screenshot to return an error. Defined here because
// the existing mock returns nil-err unconditionally.
type raitgFailScreenshotBrowser struct {
	*mockBrowserForRecording
}

func (b *raitgFailScreenshotBrowser) Screenshot(
	_ context.Context,
) ([]byte, error) {
	return nil, fmt.Errorf("screenshot disabled")
}

func TestRAITG_I18N_ScreenshotFailed_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := &raitgFailScreenshotBrowser{
		mockBrowserForRecording: newMockBrowserForRecording(),
	}
	recorder := newMockRecorderForFlow()
	testgen := &mockTestGenForRecording{available: true}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "capture")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_screenshot_failed>",
	)
}

func TestRAITG_I18N_TestGenFailed_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	recorder := newMockRecorderForFlow()
	testgen := &mockTestGenForRecording{
		available: true,
		err:       fmt.Errorf("ai backend down"),
	}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "generate_tests")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_testgen_failed>",
	)
}

func TestRAITG_I18N_VideoIntegrity_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	recorder := newMockRecorderForFlow()
	// Healthy recording — assertion records the integrity
	// message that we want to inspect.
	recorder.result = &RecordingResult{
		FilePath:   "/tmp/x.webm",
		Duration:   3 * time.Second,
		FrameCount: 90,
		FileSize:   2048,
	}
	testgen := &mockTestGenForRecording{
		available: true,
		tests: []GeneratedTest{
			{Category: "smoke", Confidence: 0.9},
		},
	}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "video_integrity")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_video_integrity>",
	)
}

// raitgNilResultRecorder returns (nil, nil) from StopRecording
// to exercise the recording-failed branch.
type raitgNilResultRecorder struct {
	*mockRecorderForFlow
}

func (r *raitgNilResultRecorder) StopRecording(
	_ context.Context,
) (*RecordingResult, error) {
	return nil, fmt.Errorf("recorder crash")
}

func TestRAITG_I18N_RecordingFailed_RoutesThroughTranslator(
	t *testing.T,
) {
	withRAITGSentinelTranslator(t)

	browser := newMockBrowserForRecording()
	recorder := &raitgNilResultRecorder{
		mockRecorderForFlow: newMockRecorderForFlow(),
	}
	testgen := &mockTestGenForRecording{
		available: true,
		tests: []GeneratedTest{
			{Category: "smoke", Confidence: 0.5},
		},
	}

	ch := buildRAITGChallenge(
		browser, recorder, testgen,
		"http://example.test", "",
	)
	res, err := ch.Execute(context.Background())
	require.NoError(t, err)
	msg := findRAITGMessage(t, res, "video_recorded")
	require.Contains(
		t, msg,
		"<TRANSLATED:challenges_userflow_recorded_ai_testgen_recording_failed>",
	)
}
