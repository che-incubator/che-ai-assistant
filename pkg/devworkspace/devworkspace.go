//
// Copyright (c) 2026 Red Hat, Inc.
// Licensed under the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspace

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/claude"
	"github.com/tolusha/che-doc-generator/pkg/config"
)

const (
	startDevWorkspaceTemplate                  = "Using MCP server `{{.CheMCPServerName}}`, start a DevWorkspace named `{{.DevWorkspaceName}}` and Claude code injected."
	deleteDevWorkspaceTemplate                 = "Using MCP server `{{.CheMCPServerName}}`, delete the DevWorkspace named `{{.DevWorkspaceName}}`."
	copyClaudeConfigTemplate                   = "Using kubectl, update lifecycle.postStart command to add `mkdir -p /home/user/.claude && cp -r /tmp/claude/* /home/user/.claude/` for DevWorkspace named `{{.DevWorkspaceName}}`."
	startClaudeTaskInDevWorkspaceTemplate      = "Using MCP server `{{.CheMCPServerName}}`, launch Claude task in DevWorkspace named `{{.DevWorkspaceName}}`: `{{.ClaudeTask}}`."
	readClaudeTaskStatusInDevWorkspaceTemplate = "Using MCP server `{{.CheMCPServerName}}`, check agent phase in DevWorkspace named `{{.DevWorkspaceName}}`. Return one word of Finished/Running/Lost/Idle"
	readClaudeTaskOutputInDevWorkspaceTemplate = "Using MCP server `{{.CheMCPServerName}}`, read Claude task output in DevWorkspace named `{{.DevWorkspaceName}}`."

	timeout = 5 * time.Minute
)

type DevWorkspace struct {
	claude        *claude.Claude
	mcpServerName string
}

func NewDevWorkspace(cfg *config.Config) *DevWorkspace {
	return &DevWorkspace{
		claude:        claude.NewClaude(cfg),
		mcpServerName: cfg.MCPServerName,
	}
}

func (dw *DevWorkspace) Start(ctx context.Context, devWorkspaceName string) error {
	_, err := dw.doRun(
		ctx,
		timeout,
		startDevWorkspaceTemplate,
		map[string]string{
			"CheMCPServerName": dw.mcpServerName,
			"DevWorkspaceName": devWorkspaceName,
		},
	)

	return err
}

func (dw *DevWorkspace) Delete(ctx context.Context, devWorkspaceName string) error {
	_, err := dw.doRun(
		ctx,
		timeout,
		deleteDevWorkspaceTemplate,
		map[string]string{
			"CheMCPServerName": dw.mcpServerName,
			"DevWorkspaceName": devWorkspaceName,
		},
	)

	return err
}

func (dw *DevWorkspace) CopyClaudeConfig(ctx context.Context, devWorkspaceName string) error {
	_, err := dw.doRun(
		ctx,
		timeout,
		copyClaudeConfigTemplate,
		map[string]string{
			"DevWorkspaceName": devWorkspaceName,
		},
	)

	return err
}

func (dw *DevWorkspace) ReadClaudeTaskStatus(ctx context.Context, devWorkspaceName string) (claude.Status, error) {
	output, err := dw.doRun(
		ctx,
		timeout,
		readClaudeTaskStatusInDevWorkspaceTemplate,
		map[string]string{
			"CheMCPServerName": dw.mcpServerName,
			"DevWorkspaceName": devWorkspaceName,
		},
	)
	if err != nil {
		return claude.StatusUnknown, err
	}

	return claude.ParseStatus(output), nil
}

func (dw *DevWorkspace) ReadClaudeTaskOutput(ctx context.Context, devWorkspaceName string) (string, error) {
	return dw.doRun(
		ctx,
		timeout,
		readClaudeTaskOutputInDevWorkspaceTemplate,
		map[string]string{
			"CheMCPServerName": dw.mcpServerName,
			"DevWorkspaceName": devWorkspaceName,
		},
	)
}

func (dw *DevWorkspace) RunClaudeTask(ctx context.Context, devWorkspaceName string, task string) error {
	_, err := dw.doRun(
		ctx,
		timeout,
		startClaudeTaskInDevWorkspaceTemplate,
		map[string]string{
			"CheMCPServerName": dw.mcpServerName,
			"DevWorkspaceName": devWorkspaceName,
			"ClaudeTask":       task,
		},
	)

	return err
}

func (dw *DevWorkspace) doRun(
	ctx context.Context,
	timeout time.Duration,
	promptTemplate string,
	data map[string]string,
) (string, error) {

	prompt, err := buildPrompt(promptTemplate, data)
	if err != nil {
		return "", err
	}

	return dw.claude.Run(ctx, timeout, prompt)
}

func buildPrompt(
	promptTemplate string,
	data map[string]string,
) (string, error) {

	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %v", err)
	}

	var prompt strings.Builder
	if err = tmpl.Execute(&prompt, data); err != nil {
		return "", fmt.Errorf("prompt template execution failed: %v", err)
	}

	return prompt.String(), nil
}
