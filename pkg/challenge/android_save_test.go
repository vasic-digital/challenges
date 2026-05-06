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
				scriptPath, api,
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
