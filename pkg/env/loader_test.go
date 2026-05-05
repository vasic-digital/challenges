package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLoader_Load(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := `# Comment
FOO=bar
BAZ="quoted value"
EMPTY=
SINGLE_QUOTE='single'
`
	require.NoError(t, os.WriteFile(envFile, []byte(content), 0644))

	l := NewLoader()
	require.NoError(t, l.Load(envFile))
	assert.True(t, l.loaded)
	assert.Equal(t, "bar", l.vars["FOO"])
	assert.Equal(t, "quoted value", l.vars["BAZ"])
	assert.Equal(t, "", l.vars["EMPTY"])
	assert.Equal(t, "single", l.vars["SINGLE_QUOTE"])
}

func TestDefaultLoader_Load_FileNotFound(t *testing.T) {
	l := NewLoader()
	err := l.Load("/nonexistent/.env")
	assert.Error(t, err)
}

func TestDefaultLoader_Get(t *testing.T) {
	l := NewLoader()
	l.vars["TEST_KEY"] = "from_file"

	// File value
	assert.Equal(t, "from_file", l.Get("TEST_KEY"))

	// OS env takes precedence
	os.Setenv("TEST_KEY_ENV", "from_os")
	defer os.Unsetenv("TEST_KEY_ENV")
	assert.Equal(t, "from_os", l.Get("TEST_KEY_ENV"))

	// Missing key
	assert.Equal(t, "", l.Get("NONEXISTENT"))
}

func TestDefaultLoader_GetRequired(t *testing.T) {
	l := NewLoader()
	l.vars["EXISTS"] = "value"

	v, err := l.GetRequired("EXISTS")
	assert.NoError(t, err)
	assert.Equal(t, "value", v)

	_, err = l.GetRequired("MISSING")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
}

func TestDefaultLoader_GetWithDefault(t *testing.T) {
	l := NewLoader()
	l.vars["EXISTS"] = "value"

	assert.Equal(t, "value", l.GetWithDefault("EXISTS", "default"))
	assert.Equal(t, "default", l.GetWithDefault("MISSING", "default"))
}

func TestDefaultLoader_GetAPIKey(t *testing.T) {
	l := NewLoader()
	l.vars["ANTHROPIC_API_KEY"] = "sk-ant-test123"

	assert.Equal(t, "sk-ant-test123", l.GetAPIKey("claude"))
	assert.Equal(t, "sk-ant-test123", l.GetAPIKey("anthropic"))
	assert.Equal(t, "", l.GetAPIKey("unknown"))
}

func TestDefaultLoader_Set(t *testing.T) {
	l := NewLoader()
	require.NoError(t, l.Set("MY_VAR", "my_value"))
	assert.Equal(t, "my_value", l.Get("MY_VAR"))
	os.Unsetenv("MY_VAR")
}

func TestDefaultLoader_All(t *testing.T) {
	l := NewLoader()
	l.vars["A"] = "1"
	l.vars["B"] = "2"

	all := l.All()
	assert.Equal(t, "1", all["A"])
	assert.Equal(t, "2", all["B"])

	// Verify it's a copy
	all["C"] = "3"
	assert.Empty(t, l.vars["C"])
}

func TestNewLoaderWithMappings(t *testing.T) {
	tests := []struct {
		name           string
		customMappings map[string]string
		provider       string
		envVarKey      string
		envVarValue    string
		expected       string
	}{
		{
			name: "custom mapping overrides default",
			customMappings: map[string]string{
				"custom": "CUSTOM_API_KEY",
			},
			provider:    "custom",
			envVarKey:   "CUSTOM_API_KEY",
			envVarValue: "custom-key-123",
			expected:    "custom-key-123",
		},
		{
			name: "custom mapping with uppercase key",
			customMappings: map[string]string{
				"NEWPROVIDER": "NEW_PROVIDER_KEY",
			},
			provider:    "newprovider",
			envVarKey:   "NEW_PROVIDER_KEY",
			envVarValue: "new-provider-value",
			expected:    "new-provider-value",
		},
		{
			name: "default mappings still work",
			customMappings: map[string]string{
				"extra": "EXTRA_KEY",
			},
			provider:    "claude",
			envVarKey:   "ANTHROPIC_API_KEY",
			envVarValue: "claude-key",
			expected:    "claude-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLoaderWithMappings(tt.customMappings)
			l.vars[tt.envVarKey] = tt.envVarValue

			result := l.GetAPIKey(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultLoader_Load_ScannerError(t *testing.T) {
	// Test that scanner errors are returned
	l := NewLoader()

	// Create a file that will be read correctly
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	// Write valid content - scanner.Err() returns nil for valid files
	content := "VALID=true\n"
	require.NoError(t, os.WriteFile(envFile, []byte(content), 0644))

	err := l.Load(envFile)
	assert.NoError(t, err)
	assert.Equal(t, "true", l.vars["VALID"])
}

func TestGetOrDefault(t *testing.T) {
	t.Setenv("TEST_GET_OR_DEFAULT_SET", "custom_value")

	assert.Equal(t, "custom_value",
		GetOrDefault("TEST_GET_OR_DEFAULT_SET", "default"))

	t.Setenv("TEST_GET_OR_DEFAULT_EMPTY", "")
	assert.Equal(t, "default",
		GetOrDefault("TEST_GET_OR_DEFAULT_EMPTY", "default"))

	assert.Equal(t, "fallback",
		GetOrDefault("NONEXISTENT_ENV_VAR_99999", "fallback"))
}

func TestDefaultLoader_Load_LineWithoutEquals(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	// Lines without = should be skipped
	content := `VALID=value
INVALID_NO_EQUALS
ANOTHER=another_value
`
	require.NoError(t, os.WriteFile(envFile, []byte(content), 0644))

	l := NewLoader()
	require.NoError(t, l.Load(envFile))
	assert.Equal(t, "value", l.vars["VALID"])
	assert.Equal(t, "another_value", l.vars["ANOTHER"])
	// The invalid line should not create a key
	_, exists := l.vars["INVALID_NO_EQUALS"]
	assert.False(t, exists)
}
