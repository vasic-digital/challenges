package userflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GoCLIAdapter implements BuildAdapter for Go projects by
// shelling out to go build, go test, and go vet commands.
type GoCLIAdapter struct {
	projectRoot string
}

// Compile-time interface check.
var _ BuildAdapter = (*GoCLIAdapter)(nil)

// NewGoCLIAdapter creates a GoCLIAdapter rooted at projectRoot.
func NewGoCLIAdapter(projectRoot string) *GoCLIAdapter {
	return &GoCLIAdapter{projectRoot: projectRoot}
}

// requireGoMod returns an error if go.mod is absent from the project root.
// go build / go test / go vet all behave differently across Go versions when
// no module is present (some versions exit 0 with a warning instead of 1),
// so callers must gate on this check before running any go command.
func (a *GoCLIAdapter) requireGoMod() error {
	gomod := filepath.Join(a.projectRoot, "go.mod")
	if _, err := os.Stat(gomod); err != nil {
		return fmt.Errorf("no go.mod found in %s: not a Go module", a.projectRoot)
	}
	return nil
}

// Build runs `go build ./...` in the project root.
// Returns a non-nil error (and Success=false) when projectRoot has no
// go.mod — modern Go otherwise treats an empty directory as "matched no
// packages" with exit 0, which silently succeeds and is never what the
// caller wanted. Fail-fast semantics are part of the adapter contract.
func (a *GoCLIAdapter) Build(
	ctx context.Context, target BuildTarget,
) (*BuildResult, error) {
	if err := a.requireGoMod(); err != nil {
		return &BuildResult{Target: target.Name, Success: false}, err
	}

	args := []string{"build"}
	if target.Task != "" {
		args = append(args, target.Task)
	} else {
		args = append(args, "./...")
	}
	args = append(args, target.Args...)

	start := time.Now()
	output, err := a.runGo(ctx, args...)
	elapsed := time.Since(start)

	return &BuildResult{
		Target:   target.Name,
		Success:  err == nil,
		Duration: elapsed,
		Output:   output,
	}, err
}

// goTestEvent represents a single event from `go test -json`.
type goTestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

// RunTests runs `go test -json ./...` and parses the JSON
// output stream into a TestResult. Requires a go.mod in projectRoot —
// see Build for the rationale.
func (a *GoCLIAdapter) RunTests(
	ctx context.Context, target TestTarget,
) (*TestResult, error) {
	if err := a.requireGoMod(); err != nil {
		return &TestResult{}, err
	}

	args := []string{"test", "-json"}
	if target.Task != "" {
		args = append(args, target.Task)
	} else {
		args = append(args, "./...")
	}
	if target.Filter != "" {
		args = append(args, "-run", target.Filter)
	}

	start := time.Now()
	output, runErr := a.runGo(ctx, args...)
	elapsed := time.Since(start)

	result := parseGoTestJSON(output, elapsed)
	return result, runErr
}

// parseGoTestJSON parses the JSON lines output of
// `go test -json` and aggregates into a TestResult.
func parseGoTestJSON(
	output string, elapsed time.Duration,
) *TestResult {
	type pkgInfo struct {
		passed  int
		failed  int
		skipped int
		cases   []TestCase
	}

	packages := make(map[string]*pkgInfo)
	scanner := bufio.NewScanner(
		strings.NewReader(output),
	)
	// `go test -json` emits one JSON object per line, and a single
	// test's output line can exceed bufio's default 64KB token
	// limit (large diffs/dumps). Without a larger buffer the
	// scanner stops at the over-long line and every later event —
	// including `fail` — is silently dropped, undercounting
	// failures (a PASS-bluff). Grow the buffer and surface any
	// remaining scan error rather than swallowing it.
	scanner.Buffer(
		make([]byte, 0, 1024*1024), 64*1024*1024,
	)

	for scanner.Scan() {
		line := scanner.Text()
		var ev goTestEvent
		if err := json.Unmarshal(
			[]byte(line), &ev,
		); err != nil {
			continue
		}

		// Only count events with a Test name (not package-
		// level events).
		if ev.Test == "" {
			continue
		}

		pkg := ev.Package
		if packages[pkg] == nil {
			packages[pkg] = &pkgInfo{}
		}
		info := packages[pkg]

		switch ev.Action {
		case "pass":
			info.passed++
			info.cases = append(info.cases, TestCase{
				Name:      ev.Test,
				ClassName: pkg,
				Duration: fmt.Sprintf(
					"%.3fs", ev.Elapsed,
				),
				Status: "passed",
			})
		case "fail":
			info.failed++
			info.cases = append(info.cases, TestCase{
				Name:      ev.Test,
				ClassName: pkg,
				Duration: fmt.Sprintf(
					"%.3fs", ev.Elapsed,
				),
				Status: "failed",
				Failure: &TestFailure{
					Message: "test failed",
					Type:    "assertion",
				},
			})
		case "skip":
			info.skipped++
			info.cases = append(info.cases, TestCase{
				Name:      ev.Test,
				ClassName: pkg,
				Duration: fmt.Sprintf(
					"%.3fs", ev.Elapsed,
				),
				Status: "skipped",
			})
		}
	}

	result := &TestResult{
		Duration: elapsed,
		Output:   output,
	}

	// A scan error (e.g. a line still exceeding the enlarged
	// buffer) means the stream was truncated and the counts below
	// would be an undercount. Record it as an error so callers do
	// not treat a partial parse as a clean pass.
	if err := scanner.Err(); err != nil {
		result.TotalErrors++
	}

	for name, info := range packages {
		total := info.passed + info.failed + info.skipped
		suite := TestSuite{
			Name:      name,
			Tests:     total,
			Failures:  info.failed,
			Skipped:   info.skipped,
			TestCases: info.cases,
		}
		result.Suites = append(result.Suites, suite)
		result.TotalTests += total
		result.TotalFailed += info.failed
		result.TotalSkipped += info.skipped
	}

	return result
}

// Lint runs `go vet ./...` in the project root. Requires a go.mod in
// projectRoot — see Build for the rationale.
func (a *GoCLIAdapter) Lint(
	ctx context.Context, target LintTarget,
) (*LintResult, error) {
	if err := a.requireGoMod(); err != nil {
		return &LintResult{Tool: "go vet", Success: false}, err
	}

	args := []string{"vet"}
	if target.Task != "" {
		args = append(args, target.Task)
	} else {
		args = append(args, "./...")
	}
	args = append(args, target.Args...)

	start := time.Now()
	output, err := a.runGo(ctx, args...)
	elapsed := time.Since(start)

	errors := 0
	if err != nil {
		// Count non-empty lines as issues.
		for _, line := range strings.Split(output, "\n") {
			if strings.TrimSpace(line) != "" {
				errors++
			}
		}
	}

	return &LintResult{
		Tool:     "go vet",
		Success:  err == nil,
		Duration: elapsed,
		Errors:   errors,
		Output:   output,
	}, err
}

// Available returns true if go.mod exists in the project root.
func (a *GoCLIAdapter) Available(
	_ context.Context,
) bool {
	return a.requireGoMod() == nil
}

// runGo executes a go command in the project root and returns
// combined output.
func (a *GoCLIAdapter) runGo(
	ctx context.Context, args ...string,
) (string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = a.projectRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf(
			"go %v: %w", args, err,
		)
	}
	return string(out), nil
}
