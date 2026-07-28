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
	"che-incubator/che-ai-assistant/pkg/config"
	"che-incubator/che-ai-assistant/pkg/mcp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

type DevWorkspace struct {
	mcpClient *mcp.Client
}

func NewDevWorkspace(cfg *config.Config) *DevWorkspace {
	return &DevWorkspace{
		mcpClient: mcp.New(cfg.MCPServerURL),
	}
}

func (dw *DevWorkspace) StartFromRepository(
	ctx context.Context,
	devWorkspaceName string,
	repoUrl string,
	branch string,
	postStartCommand string,
) error {
	log.Printf("[INFO] Starting the DevWorkspace %s", devWorkspaceName)

	_, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolCreateWorkspace,
		map[string]interface{}{
			"name":               devWorkspaceName,
			"tools":              []string{"claude-code", "tmux"},
			"repo_url":           repoUrl,
			"branch":             branch,
			"post_start_command": postStartCommand,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to start the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return nil
}

func (dw *DevWorkspace) EnsureRunning(ctx context.Context, devWorkspaceName string, timeout time.Duration) error {
	log.Printf("[INFO] Ensuring the DevWorkspace %s is running", devWorkspaceName)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for DevWorkspace %s to reach Running state", devWorkspaceName)
		case <-ticker.C:
			output, err := dw.mcpClient.CallTool(
				ctx,
				mcp.ToolGetWorkspaceStatus,
				map[string]interface{}{
					"workspace": devWorkspaceName,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to get DevWorkspace status %s: %w", devWorkspaceName, err)
			}

			var status mcp.WorkspaceStatus
			if err := json.Unmarshal([]byte(output), &status); err != nil {
				return fmt.Errorf("failed to unmarshal DevWorkspace status %s: %w", devWorkspaceName, err)
			}

			switch status.Phase {
			case "Running":
				return nil
			case "Failed":
				return fmt.Errorf("DevWorkspace %s failed to start", devWorkspaceName)
			case "Stopped":
				return fmt.Errorf("DevWorkspace %s stopped", devWorkspaceName)
			}
		}
	}
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

func (dw *DevWorkspace) WaitTaskFinished(ctx context.Context, devWorkspaceName string, timeout time.Duration) error {
	maxReadClaudeTaskStatusErrorsNumber := 3
	readClaudeTaskStatusErrorCount := 0

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
				readClaudeTaskStatusErrorCount++
				if readClaudeTaskStatusErrorCount >= maxReadClaudeTaskStatusErrorsNumber {
					return errors.Join(fmt.Errorf("failed to read task status in the DevWorkspace %s", devWorkspaceName), err)
				}

				continue
			}

			// reset error counter
			readClaudeTaskStatusErrorCount = 0

			switch status {
			case mcp.ClaudeStatusRunning:
				continue
			case mcp.ClaudeStatusFinished:
				log.Printf("[INFO] Task finished in the DevWorkspace %s, lasted %s", devWorkspaceName, time.Since(start).Round(time.Second))
				return nil
			default:
				readClaudeTaskStatusErrorCount++
				if readClaudeTaskStatusErrorCount >= maxReadClaudeTaskStatusErrorsNumber {
					return fmt.Errorf("unexpected task status %s in the DevWorkspace %s", status, devWorkspaceName)
				}
			}
		}
	}
}

func (dw *DevWorkspace) ReadClaudeTaskStatus(ctx context.Context, devWorkspaceName string) (mcp.ClaudeStatus, error) {
	output, err := dw.mcpClient.CallTool(
		ctx,
		mcp.ToolGetAgentStatus,
		map[string]interface{}{
			"workspace": devWorkspaceName,
		},
	)
	if err != nil {
		return mcp.ClaudeStatusUnknown, fmt.Errorf("failed to read Claude task status in the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	var taskStatus mcp.TaskStatus
	if err := json.Unmarshal([]byte(output), &taskStatus); err != nil {
		return "", fmt.Errorf("failed to unmarshal Claude task status from the DevWorkspace %s: %w", devWorkspaceName, err)
	}

	return mcp.ParseClaudeStatus(taskStatus.Phase), nil
}

func (dw *DevWorkspace) ReadWorkspaceAgentOutput(ctx context.Context, devWorkspaceName string) (string, error) {
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
