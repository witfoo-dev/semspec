package structuralvalidator

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

	"github.com/c360studio/semspec/tools/sandbox"
	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/payloads"
)

// CommandRunner executes a shell command and returns stdout, stderr, exit code.
// Implementations may run commands locally (os/exec) or remotely (sandbox).
type CommandRunner interface {
	Run(ctx context.Context, command string, workDir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
}

// Executor runs checklist checks against a set of modified files.
type Executor struct {
	repoPath       string
	checklistPath  string
	defaultTimeout time.Duration
	sandboxClient  *sandbox.Client // nil = local execution
}

// NewExecutor creates an Executor rooted at repoPath.
// checklistPath is relative to repoPath; defaultTimeout is used when a
// check does not declare its own Timeout.
func NewExecutor(repoPath, checklistPath string, defaultTimeout time.Duration) *Executor {
	return &Executor{
		repoPath:       repoPath,
		checklistPath:  checklistPath,
		defaultTimeout: defaultTimeout,
	}
}

// Execute runs all triggered checks for the given trigger and returns the result.
// If the checklist file is missing, it returns a passing result with a warning
// rather than an error, to allow graceful degradation in pipelines that have
// not yet been initialised.
//
// When trigger.WorktreePath is set, checks execute against that directory
// instead of the configured repoPath. The checklist is always loaded from
// repoPath (project-level config), but commands run in the worktree.
//
// When trigger.TaskID is set and a sandbox client is configured, commands
// execute inside the sandbox container rather than locally. This ensures
// agent-generated code never runs outside the sandbox boundary.
func (e *Executor) Execute(ctx context.Context, trigger *payloads.ValidationRequest) (*payloads.ValidationResult, error) {
	checklist, err := e.loadChecklist()
	if err != nil {
		if os.IsNotExist(err) {
			return &payloads.ValidationResult{
				Slug:      trigger.Slug,
				Passed:    true,
				ChecksRun: 0,
				Warning:   "No checklist.json found. Structural validation skipped.",
			}, nil
		}
		return nil, fmt.Errorf("load checklist: %w", err)
	}

	// Determine the working directory for checks. Worktree path overrides
	// the default repoPath so validation runs against agent-modified files.
	workDir := e.repoPath
	if trigger.WorktreePath != "" {
		workDir = trigger.WorktreePath
	}

	// Select command runner: sandbox for agent-generated code, local for manual validation.
	runner, err := e.runnerForTrigger(trigger)
	if err != nil {
		return nil, err
	}

	// When FilesModified is empty, run all checks (full scan mode).
	// This is the default for workflow-triggered validation where the
	// developer agent doesn't report specific files modified.
	runAll := len(trigger.FilesModified) == 0

	var results []payloads.CheckResult
	for _, check := range checklist.Checks {
		if !runAll && !matchesAny(check.Trigger, trigger.FilesModified) {
			continue
		}

		result := e.runCheckIn(ctx, check, workDir, runner)
		results = append(results, result)
	}

	// Fallback: run go test on modified packages when the checklist does not
	// already include a go-test or go-test-modified check and Go files were
	// modified. Also check the checklist itself (not just triggered results)
	// to avoid duplicating a go-test check that exists but didn't match triggers.
	// Only fires in Go projects (go.mod exists) to avoid spurious failures.
	if !hasCheckNamed(results, "go-test") && !hasCheckNamed(results, "go-test-modified") &&
		!checklistHasName(checklist, "go-test") && !checklistHasName(checklist, "go-test-modified") {
		if hasGoFiles(trigger.FilesModified) && e.isGoProjectIn(workDir) {
			goTestResult := e.runGoTestOnModifiedIn(ctx, trigger.FilesModified, workDir, runner)
			results = append(results, goTestResult)
		}
	}

	// Advisory anti-mock governance check — only when test files are present.
	if hasTestFiles(trigger.FilesModified) {
		antiMockResult := CheckAntiMock(workDir, trigger.FilesModified)
		results = append(results, antiMockResult)
	}

	passed := allRequiredPassed(results)

	return &payloads.ValidationResult{
		Slug:         trigger.Slug,
		Passed:       passed,
		ChecksRun:    len(results),
		CheckResults: results,
	}, nil
}

// runnerForTrigger returns a sandbox runner when the sandbox is configured and
// the trigger includes a TaskID. When a TaskID is present but no sandbox is
// configured, it returns an error — agent-generated code must never execute
// outside the sandbox boundary. The local runner is only used for triggers
// without a TaskID (e.g., manual validation of the main workspace).
func (e *Executor) runnerForTrigger(trigger *payloads.ValidationRequest) (CommandRunner, error) {
	if trigger.TaskID != "" {
		if e.sandboxClient == nil {
			return nil, fmt.Errorf("sandbox_url not configured but TaskID %q present — "+
				"refusing to run agent-generated code outside sandbox", trigger.TaskID)
		}
		return &sandboxRunner{client: e.sandboxClient, taskID: trigger.TaskID}, nil
	}
	return &localRunner{}, nil
}

// ---------------------------------------------------------------------------
// CommandRunner implementations
// ---------------------------------------------------------------------------

// localRunner executes commands via os/exec on the local machine.
type localRunner struct{}

func (r *localRunner) Run(ctx context.Context, command, workDir string, timeout time.Duration) (string, string, int, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := splitCommand(command)
	if len(args) == 0 {
		return "", "empty command", -1, nil
	}

	cmd := exec.CommandContext(cmdCtx, args[0], args[1:]...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdout.String(), stderr.String(), exitCode, nil
}

// sandboxRunner delegates command execution to the sandbox container.
// Commands run inside the worktree identified by taskID.
type sandboxRunner struct {
	client *sandbox.Client
	taskID string
}

func (r *sandboxRunner) Run(ctx context.Context, command, workDir string, timeout time.Duration) (string, string, int, error) {
	// The sandbox Exec routes to the worktree by taskID and sets cwd to the
	// worktree root. For checks with a WorkingDir override (e.g., "api"),
	// the caller passes the absolute path (baseDir + check.WorkingDir).
	// We need to strip the worktree prefix to get the relative subdir, then
	// prepend a cd so the sandbox runs in that subdirectory.
	//
	// However, runCheckIn already computes workDir as filepath.Join(baseDir, check.WorkingDir).
	// For sandbox execution, we pass the command with a cd prefix when workDir
	// differs from what the sandbox would use as default (worktree root).
	// The sandbox always cds into the worktree root, so any additional WorkingDir
	// is relative to that.
	cmd := command
	if workDir != "" {
		// Wrap in shell to support cd + the original command.
		cmd = fmt.Sprintf("cd %s && %s", workDir, command)
	}

	result, err := r.client.Exec(ctx, r.taskID, cmd, int(timeout.Milliseconds()))
	if err != nil {
		return "", "", -1, err
	}
	return result.Stdout, result.Stderr, result.ExitCode, nil
}

// ---------------------------------------------------------------------------
// Check execution
// ---------------------------------------------------------------------------

// hasCheckNamed returns true if any result in the slice has the given name.
func hasCheckNamed(results []payloads.CheckResult, name string) bool {
	for _, r := range results {
		if r.Name == name {
			return true
		}
	}
	return false
}

// hasGoFiles returns true if any file in the list ends with ".go".
func hasGoFiles(files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			return true
		}
	}
	return false
}

// hasTestFiles returns true if any file in the list ends with "_test.go".
func hasTestFiles(files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			return true
		}
	}
	return false
}

// checklistHasName returns true if the checklist contains a check with the given name.
func checklistHasName(cl *workflow.Checklist, name string) bool {
	for _, c := range cl.Checks {
		if c.Name == name {
			return true
		}
	}
	return false
}

// isGoProjectIn returns true if a go.mod file exists in dir.
func (e *Executor) isGoProjectIn(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

// loadChecklist reads and parses the checklist file.
func (e *Executor) loadChecklist() (*workflow.Checklist, error) {
	path := filepath.Join(e.repoPath, e.checklistPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cl workflow.Checklist
	if err := json.Unmarshal(data, &cl); err != nil {
		return nil, fmt.Errorf("parse checklist JSON: %w", err)
	}
	return &cl, nil
}

// runCheckIn executes a single check command against the given base directory.
func (e *Executor) runCheckIn(ctx context.Context, check workflow.Check, baseDir string, runner CommandRunner) payloads.CheckResult {
	timeout := e.defaultTimeout
	if check.Timeout != "" {
		if d, err := time.ParseDuration(check.Timeout); err == nil {
			timeout = d
		}
	}

	workDir := baseDir
	if check.WorkingDir != "" {
		workDir = filepath.Join(baseDir, check.WorkingDir)
	}

	start := time.Now()

	stdout, stderr, exitCode, err := runner.Run(ctx, check.Command, workDir, timeout)
	duration := time.Since(start)

	if err != nil {
		return payloads.CheckResult{
			Name:     check.Name,
			Passed:   false,
			Required: check.Required,
			Command:  check.Command,
			ExitCode: -1,
			Stderr:   fmt.Sprintf("runner error: %v", err),
			Duration: duration.String(),
		}
	}

	return payloads.CheckResult{
		Name:     check.Name,
		Passed:   exitCode == 0,
		Required: check.Required,
		Command:  check.Command,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration.String(),
	}
}

// matchesAny returns true if any file in files matches any pattern in patterns.
// Uses filepath.Match for standard glob semantics consistent with the rest of
// the Go standard library.
func matchesAny(patterns []string, files []string) bool {
	for _, pattern := range patterns {
		for _, file := range files {
			// Try both the raw file path and its base name so patterns like
			// "*.go" match files reported as "processor/foo/bar.go".
			if matched, _ := filepath.Match(pattern, file); matched {
				return true
			}
			if matched, _ := filepath.Match(pattern, filepath.Base(file)); matched {
				return true
			}
		}
	}
	return false
}

// allRequiredPassed returns true when every check marked required has passed.
// Optional failing checks do not affect the aggregate result.
func allRequiredPassed(results []payloads.CheckResult) bool {
	for _, r := range results {
		if r.Required && !r.Passed {
			return false
		}
	}
	return true
}

// DeriveGoTestPackages returns the deduplicated list of Go package paths
// (relative to repoPath, in "./pkg/path" form) that should be tested given a
// list of modified files. Only files ending in ".go" are considered. Files
// outside the module (i.e. with no directory component) map to ".".
// Returns nil when no Go files are present in filesModified.
func DeriveGoTestPackages(filesModified []string) []string {
	seen := make(map[string]struct{})
	for _, f := range filesModified {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		dir := filepath.Dir(f)
		// filepath.Dir on a bare filename returns ".".
		pkg := "./" + filepath.ToSlash(dir)
		if dir == "." {
			pkg = "."
		}
		seen[pkg] = struct{}{}
	}

	if len(seen) == 0 {
		return nil
	}

	pkgs := make([]string, 0, len(seen))
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	return pkgs
}

// runGoTestOnModifiedIn runs `go test` on the packages derived from the modified
// Go files against the given base directory.
func (e *Executor) runGoTestOnModifiedIn(ctx context.Context, filesModified []string, baseDir string, runner CommandRunner) payloads.CheckResult {
	pkgs := DeriveGoTestPackages(filesModified)
	if len(pkgs) == 0 {
		return payloads.CheckResult{
			Name:     "go-test-modified",
			Passed:   true,
			Required: true,
			Command:  "go test (skipped)",
			Stdout:   "no Go files modified",
			Duration: "0s",
		}
	}

	args := append([]string{"test"}, pkgs...)
	cmd := "go " + strings.Join(args, " ")

	start := time.Now()

	stdout, stderr, exitCode, err := runner.Run(ctx, cmd, baseDir, e.defaultTimeout)
	duration := time.Since(start)

	if err != nil {
		return payloads.CheckResult{
			Name:     "go-test-modified",
			Passed:   false,
			Required: true,
			Command:  cmd,
			ExitCode: -1,
			Stderr:   fmt.Sprintf("runner error: %v", err),
			Duration: duration.String(),
		}
	}

	return payloads.CheckResult{
		Name:     "go-test-modified",
		Passed:   exitCode == 0,
		Required: true,
		Command:  cmd,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration.String(),
	}
}

// splitCommand performs minimal whitespace-based tokenisation of a command
// string, preserving single- and double-quoted tokens.
// It is intentionally simple: it does not support escape sequences or nested
// quoting.  For complex commands the caller should wrap the command in a shell
// invocation (e.g. "sh -c '...'").
func splitCommand(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for _, r := range cmd {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
