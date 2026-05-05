package challenge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// commandRunner is a type alias for the function that runs a command.
// It allows for dependency injection in tests.
type commandRunner func(cmd *exec.Cmd) error

// defaultCommandRunner runs the command using cmd.Run().
func defaultCommandRunner(cmd *exec.Cmd) error {
	return cmd.Run()
}

// runCommand is the function used to run commands. Can be overridden in tests.
var runCommand commandRunner = defaultCommandRunner

// ShellChallenge wraps a bash script as a Challenge
// implementation. The script's exit code determines pass/fail,
// and stdout/stderr are captured as outputs.
type ShellChallenge struct {
	BaseChallenge

	// ScriptPath is the path to the bash script to execute.
	ScriptPath string

	// Args are additional arguments passed to the script.
	Args []string

	// WorkDir is the working directory for script execution.
	// If empty, the current directory is used.
	WorkDir string
}

// NewShellChallenge creates a ShellChallenge that executes the
// given bash script.
func NewShellChallenge(
	id ID,
	name, description, category string,
	deps []ID,
	scriptPath string,
	args []string,
	workDir string,
) *ShellChallenge {
	if args == nil {
		args = []string{}
	}
	return &ShellChallenge{
		BaseChallenge: NewBaseChallenge(
			id, name, description, category, deps,
		),
		ScriptPath: scriptPath,
		Args:       args,
		WorkDir:    workDir,
	}
}

// Validate checks that the script file exists and is executable.
func (s *ShellChallenge) Validate(
	ctx context.Context,
) error {
	if err := s.BaseChallenge.Validate(ctx); err != nil {
		return err
	}
	info, err := os.Stat(s.ScriptPath)
	if err != nil {
		return fmt.Errorf(
			"script %s: %w", s.ScriptPath, err,
		)
	}
	if info.IsDir() {
		return fmt.Errorf(
			"script %s: is a directory", s.ScriptPath,
		)
	}
	return nil
}

// Execute runs the bash script, captures stdout and stderr,
// and produces a Result based on the exit code.
func (s *ShellChallenge) Execute(
	ctx context.Context,
) (*Result, error) {
	start := time.Now()
	s.logInfo(
		"executing shell challenge",
		"script", s.ScriptPath,
	)

	// Apply timeout from config if set.
	if s.config != nil && s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx, s.config.Timeout,
		)
		defer cancel()
	}

	args := append([]string{s.ScriptPath}, s.Args...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.WaitDelay = 2 * time.Second

	if s.WorkDir != "" {
		cmd.Dir = s.WorkDir
	}

	// Inject environment variables from config.
	if s.config != nil && len(s.config.Environment) > 0 {
		cmd.Env = os.Environ()
		for k, v := range s.config.Environment {
			cmd.Env = append(
				cmd.Env, fmt.Sprintf("%s=%s", k, v),
			)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := runCommand(cmd)

	outputs := map[string]string{
		"stdout":    strings.TrimSpace(stdout.String()),
		"stderr":    strings.TrimSpace(stderr.String()),
		"exit_code": "0",
	}

	status := StatusPassed
	errMsg := ""

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			status = StatusTimedOut
			errMsg = "execution timed out"
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			outputs["exit_code"] = fmt.Sprintf("%d", code)
			status = StatusFailed
			errMsg = fmt.Sprintf(
				"script exited with code %d", code,
			)
		} else {
			status = StatusError
			errMsg = fmt.Sprintf("execution error: %v", err)
		}
	}

	result := s.CreateResult(
		status, start, nil, nil, outputs, errMsg,
	)
	result.RecordAction(fmt.Sprintf("ShellChallenge: executed %s, exit_code=%s, status=%s", s.ScriptPath, outputs["exit_code"], status))

	// Write output log.
	s.writeOutputLog(stdout.Bytes(), stderr.Bytes())

	// Write result JSON.
	if writeErr := s.WriteJSONResult(result); writeErr != nil {
		s.logError("failed to write result", "err", writeErr)
	}

	return result, nil
}

// writeOutputLog writes captured stdout and stderr to the
// output log file.
func (s *ShellChallenge) writeOutputLog(
	stdout, stderr []byte,
) {
	logDir := s.LogsDir()
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		s.logError("create log dir", "err", err)
		return
	}
	path := filepath.Join(logDir, "output.log")
	var buf bytes.Buffer
	buf.WriteString("=== STDOUT ===\n")
	buf.Write(stdout)
	buf.WriteString("\n=== STDERR ===\n")
	buf.Write(stderr)
	buf.WriteString("\n")
	if err := os.WriteFile(
		path, buf.Bytes(), 0o644,
	); err != nil {
		s.logError("write output log", "err", err)
	}
}
