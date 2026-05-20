package panoptic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// commandFunc is the function used to create exec.Cmd instances.
// It can be overridden in tests for dependency injection.
var commandFunc = exec.CommandContext

// CLIAdapter runs Panoptic as a subprocess to avoid pulling
// go-rod, cobra, viper, and logrus into the Challenges module.
type CLIAdapter struct {
	binaryPath string
	workDir    string
	env        map[string]string
}

// NewCLIAdapter creates a CLIAdapter pointing to the given
// Panoptic binary.
func NewCLIAdapter(binaryPath string) *CLIAdapter {
	return &CLIAdapter{
		binaryPath: binaryPath,
		env:        make(map[string]string),
	}
}

// SetWorkDir sets the working directory for Panoptic execution.
func (a *CLIAdapter) SetWorkDir(dir string) {
	a.workDir = dir
}

// SetEnv sets an environment variable for Panoptic execution.
func (a *CLIAdapter) SetEnv(key, value string) {
	a.env[key] = value
}

// Run executes `panoptic run <configPath>` as a subprocess,
// captures stdout/stderr, scans for artifacts, and returns
// a PanopticRunResult.
func (a *CLIAdapter) Run(
	ctx context.Context,
	configPath string,
	opts ...RunOption,
) (*PanopticRunResult, error) {
	cfg := resolveRunConfig(opts)

	args := []string{"run", configPath}
	if cfg.verbose {
		args = append(args, "--verbose")
	}
	if cfg.outputDir != "" {
		args = append(args, "--output", cfg.outputDir)
	}

	timeout := cfg.timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := commandFunc(execCtx, a.binaryPath, args...)
	if a.workDir != "" {
		cmd.Dir = a.workDir
	}

	cmd.Env = os.Environ()
	for k, v := range a.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range cfg.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &PanopticRunResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf(
				"panoptic execution failed: %w", err,
			)
		}
	}

	// Determine output directory for artifact scanning.
	outputDir := cfg.outputDir
	if outputDir == "" {
		outputDir = a.guessOutputDir(configPath)
	}

	if outputDir != "" {
		a.scanArtifacts(outputDir, result)
	}

	// Try to parse JSON report for structured app results.
	if result.ReportJSON != "" {
		a.parseJSONReport(result)
	}

	// If no apps from JSON report, parse stdout for app results.
	if len(result.Apps) == 0 {
		a.parseStdoutApps(result)
	}

	return result, nil
}

// Version returns the Panoptic binary version by running
// `panoptic --version`.
func (a *CLIAdapter) Version(
	ctx context.Context,
) (string, error) {
	cmd := commandFunc(ctx, a.binaryPath, "--version")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"failed to get panoptic version: %w", err,
		)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Available checks if the Panoptic binary exists and is
// executable.
func (a *CLIAdapter) Available(ctx context.Context) bool {
	info, err := os.Stat(a.binaryPath)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0111 != 0
}

// guessOutputDir tries to extract the output directory from a
// Panoptic config file by looking for the "output:" YAML key.
func (a *CLIAdapter) guessOutputDir(
	configPath string,
) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "output:") {
			val := strings.TrimPrefix(trimmed, "output:")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// scanArtifacts walks the output directory to find screenshots,
// videos, and reports.
func (a *CLIAdapter) scanArtifacts(
	dir string,
	result *PanopticRunResult,
) {
	// WalkDir reuses the fs.DirEntry produced by the directory read
	// instead of issuing a per-entry os.Lstat; the callback only needs
	// IsDir + Name, both available without a stat.
	_ = filepath.WalkDir(dir, func(
		path string, d os.DirEntry, err error,
	) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))

		switch {
		case ext == ".png" || ext == ".jpg" || ext == ".jpeg":
			result.Screenshots = append(
				result.Screenshots, path,
			)
		case ext == ".mp4" || ext == ".webm":
			result.Videos = append(result.Videos, path)
		case name == "report.html":
			result.ReportHTML = path
		case name == "report.json":
			result.ReportJSON = path
		case strings.Contains(name, "ai_error"):
			result.AIErrorReport = path
		case strings.Contains(name, "ai_test"):
			result.AIGeneratedTests = path
		case strings.Contains(name, "vision"):
			result.VisionReport = path
		}
		return nil
	})
}

// parseJSONReport reads the JSON report file and populates
// the Apps field with structured results.
func (a *CLIAdapter) parseJSONReport(
	result *PanopticRunResult,
) {
	data, err := os.ReadFile(result.ReportJSON)
	if err != nil {
		return
	}

	// Try array of app results first.
	var apps []AppResult
	if err := json.Unmarshal(data, &apps); err == nil {
		result.Apps = apps
		return
	}

	// Try single app result.
	var single AppResult
	if err := json.Unmarshal(data, &single); err == nil {
		result.Apps = []AppResult{single}
	}
}

// parseStdoutApps extracts app results from Panoptic's stdout
// by scanning for "Processing application:" and "Failed app:"
// log lines.
func (a *CLIAdapter) parseStdoutApps(result *PanopticRunResult) {
	appNames := map[string]*AppResult{}
	for _, line := range strings.Split(result.Stdout, "\n") {
		// Extract app names from processing lines.
		if idx := strings.Index(line, "Processing application:"); idx >= 0 {
			name := strings.TrimSpace(line[idx+len("Processing application:"):])
			// Strip trailing " (web)" etc.
			if paren := strings.Index(name, " ("); paren >= 0 {
				name = name[:paren]
			}
			if _, exists := appNames[name]; !exists {
				appNames[name] = &AppResult{
					Name:    name,
					Success: true,
				}
			}
		}
		// Mark failed apps.
		if idx := strings.Index(line, "Failed app:"); idx >= 0 {
			rest := line[idx+len("Failed app:"):]
			rest = strings.TrimSpace(rest)
			// Format: "AppName - error message"
			if dash := strings.Index(rest, " - "); dash >= 0 {
				name := strings.TrimSpace(rest[:dash])
				if app, exists := appNames[name]; exists {
					app.Success = false
					app.Error = strings.TrimSpace(rest[dash+3:])
				}
			}
		}
	}
	for _, app := range appNames {
		result.Apps = append(result.Apps, *app)
	}
}
