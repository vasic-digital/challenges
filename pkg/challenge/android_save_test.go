package challenge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAndroidSave_AllApiLevels(t *testing.T) {
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("SKIP-OK: #android-save-challenge-emulator - adb not available") // SKIP-OK: #android-save-challenge-emulator
	}

	emuOut, _ := exec.Command("adb", "-e", "shell", "echo", "ready").Output()
	if !strings.Contains(string(emuOut), "ready") {
		t.Skip("SKIP-OK: #android-save-challenge-emulator - no emulator connected") // SKIP-OK: #android-save-challenge-emulator
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(
		filepath.Dir(thisFile), "..", "..",
	)
	scriptPath := filepath.Join(
		repoRoot, "challenges", "scripts", "android_save_challenge.sh",
	)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Skip("SKIP-OK: #android-save-challenge-emulator - script not found") // SKIP-OK: #android-save-challenge-emulator
	}

	// The save-challenge script defaults to a project-specific APK path
	// (consumer Yole project). When this submodule is exercised in
	// isolation — i.e. by the Challenges submodule's own `make test-short`
	// rather than as part of the consumer's build — the APK does not
	// exist and `adb install` fails before any save evidence can be
	// produced. Gate on opt-in env var per the CLAUDE.md
	// "100% Decoupled" mandate: the test only runs when the consumer
	// has built its APK and explicitly opted in via APK_PATH (or
	// YOLE_ANDROID_APK_PATH for backward compatibility).
	apkPath := os.Getenv("APK_PATH")
	if apkPath == "" {
		apkPath = os.Getenv("YOLE_ANDROID_APK_PATH")
	}
	if apkPath == "" {
		t.Skip("SKIP-OK: #android-save-challenge-emulator - no APK_PATH env var set (consumer must opt in)") // SKIP-OK: #android-save-challenge-emulator
	}
	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		t.Skipf("SKIP-OK: #android-save-challenge-emulator - APK not found at %s (consumer build required)", apkPath) // SKIP-OK: #android-save-challenge-emulator
	}

	apiLevels := []string{"28", "29", "30", "31", "33", "34", "35"}
	for _, api := range apiLevels {
		api := api
		t.Run("API"+api, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(
				context.Background(), 10*time.Minute,
			)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash",
				scriptPath, api, apkPath,
			)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("AVD_NAME=yole_test_api%s", api),
				fmt.Sprintf("API_LEVEL=%s", api),
			)
			output, err := cmd.CombinedOutput()
			outStr := string(output)

			if err != nil {
				t.Errorf(
					"API %s save test failed: %v\nOutput:\n%s",
					api, err, outStr,
				)
				return
			}

			if !strings.Contains(outStr, "SAVE_VERIFIED:") {
				t.Errorf(
					"API %s: no SAVE_VERIFIED evidence in output (CONST-035)",
					api,
				)
			}
			t.Logf("API %s: PASS with evidence", api)
		})
	}
}
