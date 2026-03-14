package sandbox

import "context"

// InstallerAdapter adapts *Client to the workflow.SandboxInstaller interface.
// This avoids a circular dependency between workflow/ and tools/sandbox/.
type InstallerAdapter struct {
	client *Client
}

// NewInstallerAdapter wraps a sandbox client as a SandboxInstaller.
func NewInstallerAdapter(client *Client) *InstallerAdapter {
	if client == nil {
		return nil
	}
	return &InstallerAdapter{client: client}
}

// Install calls the sandbox /install endpoint and returns flat values
// matching the workflow.SandboxInstaller interface.
func (a *InstallerAdapter) Install(ctx context.Context, taskID, packageManager string, packages []string) (status string, stdout string, stderr string, exitCode int, err error) {
	result, err := a.client.Install(ctx, taskID, packageManager, packages)
	if err != nil {
		return "", "", "", 0, err
	}
	return result.Status, result.Stdout, result.Stderr, result.ExitCode, nil
}
