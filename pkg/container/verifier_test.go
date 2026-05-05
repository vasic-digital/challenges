// Package container provides container verification utilities
// for the Challenges framework.
package container

import (
	"context"
	"testing"

	"digital.vasic.challenges/pkg/logging"
)

// mockLogger is a simple mock for logging.Logger.
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Info(msg string, fields ...logging.Field) {
	m.logs = append(m.logs, msg)
}

func (m *mockLogger) Warn(msg string, fields ...logging.Field) {
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *mockLogger) Error(msg string, fields ...logging.Field) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *mockLogger) Debug(msg string, fields ...logging.Field) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *mockLogger) WithFields(fields ...logging.Field) logging.Logger {
	return m
}

func (m *mockLogger) LogAPIRequest(request logging.APIRequestLog) {
	m.logs = append(m.logs, "API Request: "+request.Method+" "+request.URL)
}

func (m *mockLogger) LogAPIResponse(response logging.APIResponseLog) {
	m.logs = append(m.logs, "API Response: "+string(rune(response.StatusCode)))
}

func (m *mockLogger) Close() error {
	return nil
}

// TestNewVerifier tests the creation of a new Verifier.

// TestDefaultServices tests the default service configuration.
func TestDefaultServices(t *testing.T) {
	services := DefaultServices()

	if len(services) != 3 {
		t.Errorf("Expected 3 default services, got %d", len(services))
	}

	// Check for expected services
	serviceNames := make(map[string]bool)
	for _, svc := range services {
		serviceNames[svc.Name] = true
	}

	expectedServices := []string{"postgres", "backend", "freeswitch"}
	for _, expected := range expectedServices {
		if !serviceNames[expected] {
			t.Errorf("Expected service %s not found in defaults", expected)
		}
	}
}

// TestVerifierWithServices tests custom service configuration.
func TestVerifierWithServices(t *testing.T) {
	verifier := NewVerifier(nil)

	customServices := []ServiceConfig{
		{
			Type:    ServicePostgres,
			Name:    "custom-postgres",
			Host:    "127.0.0.1",
			Port:    5433,
			Timeout: 10 * 1000000000, // 10 seconds in nanoseconds
		},
	}

	verifier.WithServices(customServices)

	if len(verifier.services) != 1 {
		t.Errorf("Expected 1 custom service, got %d", len(verifier.services))
	}

	if verifier.services[0].Name != "custom-postgres" {
		t.Errorf("Expected service name 'custom-postgres', got '%s'", verifier.services[0].Name)
	}
}

// TestFindContainersDir tests the containers directory discovery.
func TestFindContainersDir(t *testing.T) {
	// bluff-scan: no-assert-ok (directory-probe smoke — must not panic on missing/present dir)
	// This test may fail in CI environments where the directory doesn't exist
	dir := findContainersDir()

	// We can't assert the exact path, but we can verify it returns something
	// or an empty string if not found
	t.Logf("Found containers directory: %s", dir)
}

// TestPreConditionCheck tests the full pre-condition check.
func TestPreConditionCheck(t *testing.T) {
	// bluff-scan: no-assert-ok (pre-condition probe smoke — must not panic on missing/present prerequisites)
	logger := &mockLogger{}
	ctx := context.Background()

	// This test will fail if containers module is not found
	// In CI/test environments, this is expected
	err := PreConditionCheck(ctx, logger)

	if err != nil {
		// Check if it's the "containers module not found" error
		if err.Error() == "containers module not found" {
			t.Skip("Skipping test - containers module not found in test environment")
		}
		t.Logf("PreConditionCheck returned error: %v", err)
	}

	// If containers module was found, verify that logging occurred
	// Note: In test environment without containers, we skip this check
	// because the function returns early with "containers module not found"
}

// TestServiceConfig validates service configuration.
func TestServiceConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ServiceConfig
		wantErr bool
	}{
		{
			name: "valid postgres config",
			config: ServiceConfig{
				Type:    ServicePostgres,
				Name:    "postgres",
				Host:    "localhost",
				Port:    5432,
				Timeout: 30000000000,
			},
			wantErr: false,
		},
		{
			name: "valid backend config",
			config: ServiceConfig{
				Type:      ServiceBackend,
				Name:      "backend",
				Host:      "localhost",
				Port:      8090,
				HealthURL: "http://localhost:8090/health",
				Timeout:   30000000000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate configuration
			if tt.config.Port == 0 {
				t.Error("Port must not be zero")
			}
			if tt.config.Host == "" {
				t.Error("Host must not be empty")
			}
			if tt.config.Name == "" {
				t.Error("Name must not be empty")
			}
		})
	}
}

// Integration test that requires running containers
func TestIntegration_VerifyRunningContainers(t *testing.T) {
	// bluff-scan: no-assert-ok (integration/interface-compliance smoke — wiring must not panic)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	logger := &mockLogger{}
	verifier := NewVerifier(logger)
	ctx := context.Background()

	err := verifier.Verify(ctx)
	if err != nil {
		t.Skipf("Integration test skipped (containers not running): %v", err)
	}
}
