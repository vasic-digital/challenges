// Package container provides container verification utilities
// for the Challenges framework. It integrates with the
// digital.vasic.containers module for container orchestration.
package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/logging"
)

// ServiceType represents the type of containerized service.
type ServiceType string

const (
	ServicePostgres   ServiceType = "postgres"
	ServiceBackend    ServiceType = "backend"
	ServiceFreeSWITCH ServiceType = "freeswitch"
)

// ServiceConfig defines a service to be verified.
type ServiceConfig struct {
	Type      ServiceType
	Name      string
	Host      string
	Port      int
	HealthURL string
	Timeout   time.Duration
}

// DefaultServices returns the default example services.
func DefaultServices() []ServiceConfig {
	return []ServiceConfig{
		{
			Type:    ServicePostgres,
			Name:    "postgres",
			Host:    "localhost",
			Port:    5432,
			Timeout: 30 * time.Second,
		},
		{
			Type:      ServiceBackend,
			Name:      "backend",
			Host:      "localhost",
			Port:      8090,
			HealthURL: "http://localhost:8090/health",
			Timeout:   30 * time.Second,
		},
		{
			Type:    ServiceFreeSWITCH,
			Name:    "freeswitch",
			Host:    "localhost",
			Port:    5060,
			Timeout: 30 * time.Second,
		},
	}
}

// Verifier checks that container services are running and healthy.
type Verifier struct {
	services []ServiceConfig
	logger   logging.Logger
}

// NewVerifier creates a new Verifier with default services.
func NewVerifier(logger logging.Logger) *Verifier {
	return &Verifier{
		services: DefaultServices(),
		logger:   logger,
	}
}

// WithServices sets custom services to verify.
func (v *Verifier) WithServices(services []ServiceConfig) *Verifier {
	v.services = services
	return v
}

// Verify checks all configured services.
func (v *Verifier) Verify(ctx context.Context) error {
	if v.logger != nil {
		v.logger.Info("Starting container service verification...")
	}

	for _, svc := range v.services {
		if err := v.verifyService(ctx, svc); err != nil {
			return fmt.Errorf("service %s verification failed: %w", svc.Name, err)
		}
	}

	if v.logger != nil {
		v.logger.Info("All container services verified successfully")
	}
	return nil
}

// verifyService checks a single service.
func (v *Verifier) verifyService(ctx context.Context, svc ServiceConfig) error {
	if v.logger != nil {
		v.logger.Info(fmt.Sprintf("Verifying service: %s (%s:%d)", svc.Name, svc.Host, svc.Port))
	}

	// Check TCP connectivity
	if err := v.checkTCP(ctx, svc.Host, svc.Port, svc.Timeout); err != nil {
		return fmt.Errorf("TCP check failed: %w", err)
	}

	// For backend, also check HTTP health endpoint
	if svc.Type == ServiceBackend && svc.HealthURL != "" {
		if err := v.checkHTTP(ctx, svc.HealthURL, svc.Timeout); err != nil {
			return fmt.Errorf("HTTP health check failed: %w", err)
		}
	}

	if v.logger != nil {
		v.logger.Info(fmt.Sprintf("Service %s is healthy", svc.Name))
	}
	return nil
}

// checkTCP verifies TCP connectivity.
func (v *Verifier) checkTCP(ctx context.Context, host string, port int, timeout time.Duration) error {
	// Use a simple TCP dial check
	// In production, this would use proper connection checking
	address := fmt.Sprintf("%s:%d", host, port)

	// Create a timeout context
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Simple TCP check using /dev/tcp if available (bash feature)
	cmd := fmt.Sprintf("timeout %d bash -c 'echo > /dev/tcp/%s/%d' 2>/dev/null",
		int(timeout.Seconds()), host, port)

	if err := execCommand(ctx, cmd); err != nil {
		return fmt.Errorf("cannot connect to %s: %w", address, err)
	}

	return nil
}

// checkHTTP verifies HTTP health endpoint.
func (v *Verifier) checkHTTP(ctx context.Context, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := fmt.Sprintf("curl -sf %s | grep -q 'status.*ok'", url)

	if err := execCommand(ctx, cmd); err != nil {
		return fmt.Errorf("health check failed for %s: %w", url, err)
	}

	return nil
}

// execCommand executes a shell command using the OS exec package.
func execCommand(ctx context.Context, cmd string) error {
	args := []string{"-c", cmd}
	execCmd := exec.CommandContext(ctx, "bash", args...)
	execCmd.Stdout = nil
	execCmd.Stderr = nil
	return execCmd.Run()
}

// PreConditionCheck performs the full pre-condition verification.
// This is the main entry point used by the challenges framework.
func PreConditionCheck(ctx context.Context, logger logging.Logger) error {
	// 1. Check if .env file exists in Containers module
	containersDir := findContainersDir()
	if containersDir == "" {
		return fmt.Errorf("containers module not found")
	}

	envFile := filepath.Join(containersDir, ".env")
	if _, err := os.Stat(envFile); err != nil {
		if logger != nil {
			logger.Warn("WARNING: .env file not found in Containers module")
			logger.Info("Containers will boot on LOCAL HOST")
		}
	} else {
		content, err := os.ReadFile(envFile)
		if err != nil {
			return fmt.Errorf("cannot read .env file: %w", err)
		}

		if len(strings.TrimSpace(string(content))) == 0 {
			if logger != nil {
				logger.Warn("WARNING: .env file is empty")
				logger.Info("Containers will boot on LOCAL HOST")
			}
		} else {
			if logger != nil {
				logger.Info("Found .env file in Containers module")
			}

			// Check for remote configuration
			if strings.Contains(string(content), "CONTAINERS_REMOTE_ENABLED=true") {
				if logger != nil {
					logger.Info("Remote container distribution is ENABLED")
				}
			}
		}
	}

	// 2. Verify container services are running
	verifier := NewVerifier(logger)
	if err := verifier.Verify(ctx); err != nil {
		return fmt.Errorf("container verification failed: %w", err)
	}

	return nil
}

// findContainersDir attempts to find the Containers module directory.
func findContainersDir() string {
	// Try common paths
	paths := []string{
		"tools/containers",
		"../tools/containers",
		"../../tools/containers",
		"./containers",
		"../containers",
	}

	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}
