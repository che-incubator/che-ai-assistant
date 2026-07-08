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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/claude"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/mcp"
)

const (
	copyClaudeConfigTemplate = "Using kubectl, update lifecycle.postStart command to add `mkdir -p /home/user/.claude && cp -r /tmp/claude/* /home/user/.claude/` for DevWorkspace named `{{.DevWorkspaceName}}`."

	execSuccessMarker = "!!__SUCCESS__!!"
	execFailedMarker  = "!!__FAILED__!!"

	timeout = 5 * time.Minute
)

type DevWorkspace struct {
	mcpClient *mcp.Client
	claude    *claude.Claude
}

func NewDevWorkspace(cfg *config.Config) *DevWorkspace {
	return &DevWorkspace{
		mcpClient: mcp.New(cfg.MCPServerURL),
		claude:    claude.New(cfg),
	}
}

func (dw *DevWorkspace) Start(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Starting the DevWorkspace %s", devWorkspaceName)

	_, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolCreateWorkspace,
		map[string]interface{}{
			"name":  devWorkspaceName,
			"tools": []string{"claude-code", "tmux"},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to start the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return nil
}

func (dw *DevWorkspace) StartWithRepository(
	ctx context.Context,
	devWorkspaceName string,
	repoUrl string,
	branch string,
) error {
	log.Printf("[INFO] Starting the DevWorkspace %s", devWorkspaceName)

	_, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolCreateWorkspace,
		map[string]interface{}{
			"name":     devWorkspaceName,
			"tools":    []string{"claude-code", "tmux"},
			"repo_url": repoUrl,
			"branch":   branch,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to start the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return nil
}

func (dw *DevWorkspace) Delete(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Deleting the DevWorkspace %s", devWorkspaceName)

	_, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolDeleteWorkspace,
		map[string]interface{}{
			"workspace": devWorkspaceName,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return nil
}

func (dw *DevWorkspace) Exec(ctx context.Context, devWorkspaceName string, command string) error {
	log.Printf("[INFO] Executing command in the DevWorkspace %s", devWorkspaceName)

	wrappedCommand := command + " && echo " + execSuccessMarker + " || echo " + execFailedMarker

	_, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolExecInWorkspace,
		map[string]interface{}{
			"workspace":       devWorkspaceName,
			"command":         wrappedCommand,
			"timeout_seconds": 0,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to exec in the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return nil
}

func (dw *DevWorkspace) WaitTaskFinished(ctx context.Context, devWorkspaceName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for task to finish in the DevWorkspace %s", devWorkspaceName)
		case <-ticker.C:
			log.Printf("[INFO] Waiting for task to finish in the DevWorkspace %s (elapsed: %s)", devWorkspaceName, time.Since(start).Round(time.Second))
			status, err := dw.ReadClaudeTaskStatus(ctx, devWorkspaceName)
			if err != nil {
				return errors.Join(fmt.Errorf("failed to read task status in the DevWorkspace %s", devWorkspaceName), err)
			}

			switch status {
			case claude.StatusRunning:
				continue
			case claude.StatusFinished:
				log.Printf("[INFO] Task finished in the DevWorkspace %s, lasted %s", devWorkspaceName, time.Since(start).Round(time.Second))
				return nil
			default:
				return fmt.Errorf("unexpected task status %s in the DevWorkspace %s", status, devWorkspaceName)
			}
		}
	}
}

func (dw *DevWorkspace) WaitExecFinished(ctx context.Context, devWorkspaceName string) error {
	maxReadOutputErrorsNumber := 3
	readOutputErrorsCount := 0

	start := time.Now()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for task to finish in the DevWorkspace %s", devWorkspaceName)
		case <-ticker.C:
			log.Printf("[INFO] Waiting for task to finish in the DevWorkspace %s (elapsed: %s)", devWorkspaceName, time.Since(start).Round(time.Second))
			output, err := dw.ReadWorkspaceOutput(ctx, devWorkspaceName)

			if err != nil {
				readOutputErrorsCount++
				if readOutputErrorsCount > maxReadOutputErrorsNumber {
					return fmt.Errorf("failed to get output from the DevWorkspace %s: %w", devWorkspaceName, err)
				}
			}

			if strings.Contains(output, execFailedMarker) {
				return fmt.Errorf("failed executing the command in the DevWorkspace %s: %w", devWorkspaceName, err)
			}

			if strings.Contains(output, execSuccessMarker) {
				log.Printf("[INFO] Task finished in the DevWorkspace %s, lasted %s", devWorkspaceName, time.Since(start).Round(time.Second))
				return nil
			}
		}
	}
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
	output, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolGetAgentStatus,
		map[string]interface{}{
			"workspace": devWorkspaceName,
		},
	)
	if err != nil {
		return claude.StatusUnknown, fmt.Errorf("failed to read Claude task status in the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	var taskStatus mcp.TaskStatus
	if err := json.Unmarshal([]byte(output), &taskStatus); err != nil {
		return "", fmt.Errorf("failed to unmarshal Claude task status from the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return claude.ParseStatus(taskStatus.Phase), nil
}

func (dw *DevWorkspace) ReadWorkspaceOutput(ctx context.Context, devWorkspaceName string) (string, error) {
	log.Printf("[INFO] Reading Claude task output in the DevWorkspace %s", devWorkspaceName)

	output, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolGetAgentOutput,
		map[string]interface{}{
			"workspace": devWorkspaceName,
			"lines":     100,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to read Claude task output in the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	var taskOutput mcp.TaskOutput
	if err := json.Unmarshal([]byte(output), &taskOutput); err != nil {
		return "", fmt.Errorf("failed to unmarshal Claude task output from the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return taskOutput.Output, nil
}

func (dw *DevWorkspace) RunClaudeTask(ctx context.Context, devWorkspaceName string, task string) error {
	log.Printf("[INFO] Running Claude task in the DevWorkspace %s", devWorkspaceName)

	_, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolLaunchCodingAgent,
		map[string]interface{}{
			"workspace":  devWorkspaceName,
			"task":       task,
			"agent_type": mcp.AgentClaude,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to run task in the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return nil
}

func (dw *DevWorkspace) CopyClaudeConfigInDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Copying Claude config in the DevWorkspace %s", devWorkspaceName)

	err := dw.CopyClaudeConfig(ctx, devWorkspaceName)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to copy Claude config in the DevWorkspace %s", devWorkspaceName), err)
	}

	return nil
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
