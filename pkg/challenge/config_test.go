package challenge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig_DifferentIDs(t *testing.T) {
	tests := []struct {
		name string
		id   ID
	}{
		{"simple", "simple-id"},
		{"with-dashes", "my-test-challenge"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig(tt.id)
			assert.Equal(t, tt.id, cfg.ChallengeID)
		})
	}
}

func TestConfig_GetEnv_Found(t *testing.T) {
	cfg := &Config{
		Environment: map[string]string{
			"API_URL": "http://localhost:8080",
			"TOKEN":   "abc123",
		},
	}
	assert.Equal(t, "http://localhost:8080", cfg.GetEnv("API_URL", "default"))
	assert.Equal(t, "abc123", cfg.GetEnv("TOKEN", "default"))
}

func TestConfig_GetEnv_NotFound(t *testing.T) {
	cfg := &Config{
		Environment: map[string]string{
			"API_URL": "http://localhost:8080",
		},
	}
	assert.Equal(t, "default_value", cfg.GetEnv("MISSING_KEY", "default_value"))
}

func TestConfig_GetEnv_NilEnvironment(t *testing.T) {
	cfg := &Config{
		Environment: nil,
	}
	assert.Equal(t, "fallback", cfg.GetEnv("ANY_KEY", "fallback"))
}

func TestConfig_GetEnv_EmptyEnvironment(t *testing.T) {
	cfg := &Config{
		Environment: map[string]string{},
	}
	assert.Equal(t, "fallback", cfg.GetEnv("ANY_KEY", "fallback"))
}

func TestConfig_GetEnv_EmptyValue(t *testing.T) {
	cfg := &Config{
		Environment: map[string]string{
			"EMPTY_VAR": "",
		},
	}
	// Empty string is a valid value, should not fall back
	assert.Equal(t, "", cfg.GetEnv("EMPTY_VAR", "fallback"))
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		ChallengeID: "my-challenge",
		ResultsDir:  "/tmp/results",
		LogsDir:     "/tmp/logs",
		Timeout:     30 * time.Second,
		Verbose:     true,
		Environment: map[string]string{"KEY": "VALUE"},
		Dependencies: map[ID]string{
			"dep-1": "/tmp/dep1/result.json",
		},
	}

	assert.Equal(t, ID("my-challenge"), cfg.ChallengeID)
	assert.Equal(t, "/tmp/results", cfg.ResultsDir)
	assert.Equal(t, "/tmp/logs", cfg.LogsDir)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.True(t, cfg.Verbose)
	assert.Len(t, cfg.Environment, 1)
	assert.Len(t, cfg.Dependencies, 1)
}
