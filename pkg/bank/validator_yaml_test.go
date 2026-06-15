// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Anti-bluff regression tests for ValidateFile format + root-key
// parity with LoadFile. Constitution §11.4 / §11.4.1: the validator
// MUST validate every bank file that LoadFile accepts, and MUST NOT
// silently skip validation of HelixQA `test_cases` banks (where a
// duplicate ID / missing name would otherwise pass unnoticed — a
// validation-layer PASS-bluff).
//
// These tests reproduce the pre-fix defect on the CURRENT code (RED):
//   1. ValidateFile rejected a valid YAML bank with a "json" parse
//      error even though LoadFile loads YAML banks fine.
//   2. ValidateFile ignored the `test_cases` root key (folded into
//      Challenges by parseBankFile), so a HelixQA bank with duplicate
//      IDs / missing names validated clean.

package bank

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateFile_YAMLBank_Valid proves ValidateFile accepts a valid
// YAML bank — the same file LoadFile would load without error. Pre-fix
// this FAILS because ValidateFile only ran json.Unmarshal and reported
// a spurious "json" parse error on valid YAML.
func TestValidateFile_YAMLBank_Valid(t *testing.T) {
	dir := t.TempDir()
	yamlData := []byte(`version: "1.0"
name: yaml-bank
challenges:
  - id: ch-1
    name: Test 1
  - id: ch-2
    name: Test 2
`)
	path := filepath.Join(dir, "valid.yaml")
	require.NoError(t, os.WriteFile(path, yamlData, 0644))

	// Sanity: LoadFile accepts this YAML — proving the format is valid.
	require.NoError(t, New().LoadFile(path))

	errors := ValidateFile(path)
	assert.Empty(t, errors,
		"valid YAML bank must validate clean (parity with LoadFile); got %v", errors)
}

// TestValidateFile_TestCasesRootKey_DuplicateIDDetected proves
// ValidateFile validates the HelixQA `test_cases` root key (which
// parseBankFile folds into Challenges). The bank has a duplicate ID;
// pre-fix ValidateFile ignored test_cases entirely and reported no
// errors — a validation-layer PASS-bluff letting a malformed bank
// through.
func TestValidateFile_TestCasesRootKey_DuplicateIDDetected(t *testing.T) {
	dir := t.TempDir()
	yamlData := []byte(`version: "1.0"
name: helixqa-bank
test_cases:
  - id: dup
    name: First
  - id: dup
    name: Second
`)
	path := filepath.Join(dir, "helixqa.yaml")
	require.NoError(t, os.WriteFile(path, yamlData, 0644))

	errors := ValidateFile(path)

	var sawDuplicate bool
	for _, e := range errors {
		if e.Field == "id" && e.Index == 1 {
			sawDuplicate = true
		}
	}
	assert.True(t, sawDuplicate,
		"ValidateFile must detect the duplicate ID in a test_cases bank; got %v", errors)
}

// TestValidateFile_TestCasesRootKey_MissingNameDetected proves
// ValidateFile flags a missing name in a JSON `test_cases` bank.
// Pre-fix this passed clean because test_cases was never read.
func TestValidateFile_TestCasesRootKey_MissingNameDetected(t *testing.T) {
	dir := t.TempDir()
	jsonData := []byte(`{"version":"1.0","test_cases":[{"id":"ch-1"}]}`)
	path := filepath.Join(dir, "helixqa.json")
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	errors := ValidateFile(path)

	var sawMissingName bool
	for _, e := range errors {
		if e.Field == "name" {
			sawMissingName = true
		}
	}
	assert.True(t, sawMissingName,
		"ValidateFile must flag the missing name in a test_cases bank; got %v", errors)
}
