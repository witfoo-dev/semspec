package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInstaller implements SandboxInstaller for testing.
type mockInstaller struct {
	calls    []installCall
	status   string
	stdout   string
	stderr   string
	exitCode int
	err      error
}

type installCall struct {
	TaskID         string
	PackageManager string
	Packages       []string
}

func (m *mockInstaller) Install(_ context.Context, taskID, packageManager string, packages []string) (string, string, string, int, error) {
	m.calls = append(m.calls, installCall{
		TaskID:         taskID,
		PackageManager: packageManager,
		Packages:       packages,
	})
	return m.status, m.stdout, m.stderr, m.exitCode, m.err
}

func TestSandboxActionExecutor_Supports(t *testing.T) {
	exec := NewSandboxActionExecutor(&mockInstaller{})
	assert.True(t, exec.Supports(ActionInstallPackage))
	assert.False(t, exec.Supports(ActionSuggestAlternative))
	assert.False(t, exec.Supports(ActionNone))
	assert.False(t, exec.Supports("unknown"))
}

func TestSandboxActionExecutor_InstallSuccess(t *testing.T) {
	installer := &mockInstaller{
		status: "installed",
		stdout: "Setting up cargo (1.70.0)...",
	}
	exec := NewSandboxActionExecutor(installer)

	result, err := exec.Execute(context.Background(), "task-1", &AnswerAction{
		Type: ActionInstallPackage,
		Parameters: map[string]string{
			"packages":        "cargo",
			"package_manager": "apt",
		},
	})

	require.NoError(t, err)
	assert.Contains(t, result, "installed cargo via apt")
	assert.Contains(t, result, "Setting up cargo")

	require.Len(t, installer.calls, 1)
	assert.Equal(t, "task-1", installer.calls[0].TaskID)
	assert.Equal(t, "apt", installer.calls[0].PackageManager)
	assert.Equal(t, []string{"cargo"}, installer.calls[0].Packages)
}

func TestSandboxActionExecutor_InstallMultiplePackages(t *testing.T) {
	installer := &mockInstaller{status: "installed"}
	exec := NewSandboxActionExecutor(installer)

	_, err := exec.Execute(context.Background(), "task-1", &AnswerAction{
		Type: ActionInstallPackage,
		Parameters: map[string]string{
			"packages":        "cargo, rustfmt",
			"package_manager": "npm",
		},
	})

	require.NoError(t, err)
	require.Len(t, installer.calls, 1)
	assert.Equal(t, []string{"cargo", "rustfmt"}, installer.calls[0].Packages)
	assert.Equal(t, "npm", installer.calls[0].PackageManager)
}

func TestSandboxActionExecutor_DefaultsToApt(t *testing.T) {
	installer := &mockInstaller{status: "installed"}
	exec := NewSandboxActionExecutor(installer)

	_, err := exec.Execute(context.Background(), "task-1", &AnswerAction{
		Type:       ActionInstallPackage,
		Parameters: map[string]string{"packages": "jq"},
	})

	require.NoError(t, err)
	assert.Equal(t, "apt", installer.calls[0].PackageManager)
}

func TestSandboxActionExecutor_InstallFailure(t *testing.T) {
	installer := &mockInstaller{
		status:   "failed",
		stderr:   "E: Unable to locate package foobar",
		exitCode: 100,
	}
	exec := NewSandboxActionExecutor(installer)

	_, err := exec.Execute(context.Background(), "task-1", &AnswerAction{
		Type:       ActionInstallPackage,
		Parameters: map[string]string{"packages": "foobar"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 100")
	assert.Contains(t, err.Error(), "Unable to locate package")
}

func TestSandboxActionExecutor_MissingPackages(t *testing.T) {
	exec := NewSandboxActionExecutor(&mockInstaller{})

	_, err := exec.Execute(context.Background(), "task-1", &AnswerAction{
		Type:       ActionInstallPackage,
		Parameters: map[string]string{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "packages")
}

func TestSandboxActionExecutor_InstallerError(t *testing.T) {
	installer := &mockInstaller{err: fmt.Errorf("connection refused")}
	exec := NewSandboxActionExecutor(installer)

	_, err := exec.Execute(context.Background(), "task-1", &AnswerAction{
		Type:       ActionInstallPackage,
		Parameters: map[string]string{"packages": "cargo"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestActionDispatcher_RoutesToCorrectExecutor(t *testing.T) {
	installer := &mockInstaller{status: "installed"}
	exec := NewSandboxActionExecutor(installer)
	dispatcher := NewActionDispatcher(nil, exec)

	result, err := dispatcher.Execute(context.Background(), "task-1", &AnswerAction{
		Type:       ActionInstallPackage,
		Parameters: map[string]string{"packages": "jq"},
	})

	require.NoError(t, err)
	assert.Contains(t, result, "installed jq")
}

func TestActionDispatcher_NoExecutorFound(t *testing.T) {
	dispatcher := NewActionDispatcher(nil)

	_, err := dispatcher.Execute(context.Background(), "task-1", &AnswerAction{
		Type: ActionInstallPackage,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no executor registered")
}

func TestActionDispatcher_SkipsNone(t *testing.T) {
	dispatcher := NewActionDispatcher(nil)

	result, err := dispatcher.Execute(context.Background(), "task-1", &AnswerAction{
		Type: ActionNone,
	})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestActionDispatcher_SkipsSuggestAlternative(t *testing.T) {
	dispatcher := NewActionDispatcher(nil)

	result, err := dispatcher.Execute(context.Background(), "task-1", &AnswerAction{
		Type: ActionSuggestAlternative,
	})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestActionDispatcher_NilAction(t *testing.T) {
	dispatcher := NewActionDispatcher(nil)

	result, err := dispatcher.Execute(context.Background(), "task-1", nil)

	require.NoError(t, err)
	assert.Empty(t, result)
}
