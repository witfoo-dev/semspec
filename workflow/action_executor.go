package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// ActionExecutor executes machine-readable actions attached to answers.
// Implementations are package-manager or environment-specific.
type ActionExecutor interface {
	// Execute runs the given action in the context of the specified task.
	// Returns a human-readable result message and any error.
	Execute(ctx context.Context, taskID string, action *AnswerAction) (string, error)

	// Supports reports whether this executor handles the given action type.
	Supports(actionType string) bool
}

// ActionDispatcher routes actions to the appropriate executor.
type ActionDispatcher struct {
	executors []ActionExecutor
	logger    *slog.Logger
}

// NewActionDispatcher creates a dispatcher that routes actions to registered executors.
func NewActionDispatcher(logger *slog.Logger, executors ...ActionExecutor) *ActionDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &ActionDispatcher{
		executors: executors,
		logger:    logger,
	}
}

// Execute finds a supporting executor and runs the action.
// Returns the result message or an error if no executor supports the action type
// or execution fails.
func (d *ActionDispatcher) Execute(ctx context.Context, taskID string, action *AnswerAction) (string, error) {
	if action == nil {
		return "", nil
	}
	if action.Type == ActionNone || action.Type == ActionSuggestAlternative {
		// These are informational — no execution needed.
		return "", nil
	}

	for _, exec := range d.executors {
		if exec.Supports(action.Type) {
			d.logger.Info("Executing answer action",
				"type", action.Type,
				"task_id", taskID,
				"parameters", action.Parameters,
			)
			result, err := exec.Execute(ctx, taskID, action)
			if err != nil {
				d.logger.Error("Action execution failed",
					"type", action.Type,
					"task_id", taskID,
					"error", err,
				)
				return "", err
			}
			d.logger.Info("Action executed successfully",
				"type", action.Type,
				"task_id", taskID,
				"result", result,
			)
			return result, nil
		}
	}

	return "", fmt.Errorf("no executor registered for action type %q", action.Type)
}

// SandboxInstaller is an interface for installing packages via the sandbox.
// This avoids a direct dependency on tools/sandbox from workflow/.
type SandboxInstaller interface {
	Install(ctx context.Context, taskID, packageManager string, packages []string) (status string, stdout string, stderr string, exitCode int, err error)
}

// SandboxActionExecutor executes actions by calling the sandbox API.
type SandboxActionExecutor struct {
	installer SandboxInstaller
}

// NewSandboxActionExecutor creates an executor backed by a sandbox installer.
// Returns nil if installer is nil (sandbox disabled).
func NewSandboxActionExecutor(installer SandboxInstaller) *SandboxActionExecutor {
	if installer == nil {
		return nil
	}
	return &SandboxActionExecutor{installer: installer}
}

// Supports reports whether this executor handles the given action type.
func (e *SandboxActionExecutor) Supports(actionType string) bool {
	return actionType == ActionInstallPackage
}

// Execute runs install_package by calling the sandbox /install endpoint.
// Expected parameters:
//   - packages: comma-separated list of package names (required)
//   - package_manager: apt, npm, pip, go (defaults to "apt")
func (e *SandboxActionExecutor) Execute(ctx context.Context, taskID string, action *AnswerAction) (string, error) {
	if action.Type != ActionInstallPackage {
		return "", fmt.Errorf("unsupported action type %q", action.Type)
	}

	packagesStr := action.Parameters["packages"]
	if packagesStr == "" {
		return "", fmt.Errorf("install_package action requires 'packages' parameter")
	}

	packages := strings.Split(packagesStr, ",")
	for i := range packages {
		packages[i] = strings.TrimSpace(packages[i])
	}

	pm := action.Parameters["package_manager"]
	if pm == "" {
		pm = "apt"
	}

	status, stdout, stderr, exitCode, err := e.installer.Install(ctx, taskID, pm, packages)
	if err != nil {
		return "", fmt.Errorf("sandbox install: %w", err)
	}

	if status != "installed" {
		return "", fmt.Errorf("install failed (exit %d): %s", exitCode, stderr)
	}

	result := fmt.Sprintf("installed %s via %s", packagesStr, pm)
	if stdout != "" {
		result += "\n" + stdout
	}
	return result, nil
}
