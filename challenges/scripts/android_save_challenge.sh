#!/bin/bash
# android_save_challenge.sh - Run save tests on a specific API level emulator
# Anti-bluff: requires SAVE_VERIFIED positive evidence in output

API="${1:-28}"
AVD="${AVD_NAME:-yole_test_api${API}}"
APK="${2:-androidApp/build/outputs/apk/debug/androidApp-debug.apk}"
RESULTS_FILE="/tmp/yole_save_api${API}_results.txt"

echo "=== Android Save Challenge - API ${API} ==="
echo "AVD: ${AVD}"
echo "APK: ${APK}"

adb -e wait-for-device
adb -e install -r "${APK}"
adb -e shell am instrument -w -r \
    -e class digital.vasic.yole.android.SaveTests \
    digital.vasic.yole.android.test/androidx.test.runner.AndroidJUnitRunner \
    > "${RESULTS_FILE}" 2>&1

if grep -q "FAILURES" "${RESULTS_FILE}"; then
    echo "FAIL: Tests failed on API ${API}"
    cat "${RESULTS_FILE}"
    exit 1
fi

EVIDENCE=$(grep -oE 'SAVE_VERIFIED: [0-9]+ bytes' "${RESULTS_FILE}" | head -5)
if [ -z "${EVIDENCE}" ]; then
    echo "FAIL: No SAVE_VERIFIED evidence (CONST-035 violation) on API ${API}"
    exit 1
fi

echo "${EVIDENCE}"
echo "PASS: API ${API} save tests verified"
exit 0
