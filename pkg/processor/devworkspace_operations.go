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

package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/claude"
)

func (p *TaskProcessor) startDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Starting the DevWorkspace %s", devWorkspaceName)

	err := p.devWorkspace.Start(ctx, devWorkspaceName)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to start DevWorkspace %s", devWorkspaceName), err)
	}

	return nil
}

func (p *TaskProcessor) copyClaudeConfigInDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Copying Claude config in the DevWorkspace %s", devWorkspaceName)

	err := p.devWorkspace.CopyClaudeConfig(ctx, devWorkspaceName)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to copy Claude config in the DevWorkspace %s", devWorkspaceName), err)
	}

	return nil
}

func (p *TaskProcessor) runTaskInDevWorkspace(ctx context.Context, task string, devWorkspaceName string) error {
	log.Printf("[INFO] Running task in the DevWorkspace %s", devWorkspaceName)

	err := p.devWorkspace.RunClaudeTask(ctx, devWorkspaceName, task)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to run task in the DevWorkspace %s", devWorkspaceName), err)
	}

	return nil
}

func (p *TaskProcessor) deleteDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Deleting the DevWorkspace %s", devWorkspaceName)

	err := p.devWorkspace.Delete(ctx, devWorkspaceName)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to delete the DevWorkspace %s", devWorkspaceName), err)
	}

	return nil
}

func (p *TaskProcessor) readTaskOutputInDevWorkspace(ctx context.Context, devWorkspaceName string) (string, error) {
	log.Printf("[INFO] Reading task output in the DevWorkspace %s", devWorkspaceName)

	output, err := p.devWorkspace.ReadClaudeTaskOutput(ctx, devWorkspaceName)
	if err != nil {
		return "", errors.Join(fmt.Errorf("failed to read task output in the DevWorkspace %s", devWorkspaceName), err)
	}

	return output, nil
}

func (p *TaskProcessor) waiteTaskFinishedInDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Waiting for task to finish in the DevWorkspace %s", devWorkspaceName)

	ctx, cancel := context.WithTimeout(ctx, p.taskTimeout)
	defer cancel()

	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for task to finish in the DevWorkspace %s", devWorkspaceName)
		case <-ticker.C:
			status, err := p.devWorkspace.ReadClaudeTaskStatus(ctx, devWorkspaceName)
			if err != nil {
				return errors.Join(fmt.Errorf("failed to read task status in the DevWorkspace %s", devWorkspaceName), err)
			}

			switch status {
			case claude.TaskStatusRunning:
				continue
			case claude.TaskStatusCompleted:
				return nil
			default:
				return fmt.Errorf("unexpected task status %s in the DevWorkspace %s", status, devWorkspaceName)
			}
		}
	}
}
