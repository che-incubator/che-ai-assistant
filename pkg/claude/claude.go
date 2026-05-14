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

package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/config"
)

type Output struct {
	Result string `json:"result"`
}

type Claude struct {
	outputDir string
}

type Status string

const (
	TaskStatusRunning  Status = "Running"
	TaskStatusFinished Status = "Finished"
	TaskStatusUnknown  Status = "Unknown"
)

func NewClaude(cfg *config.Config) *Claude {
	return &Claude{outputDir: cfg.ClaudeOutputDir}
}

func (r *Claude) Run(ctx context.Context, timeout time.Duration, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	outputFile := filepath.Join(r.outputDir, fmt.Sprintf("claude-output-%d.json", time.Now().UnixNano()))

	log.Printf("[INFO] Claude started, prompt:")
	log.Printf("%s", prompt)
	log.Printf("[INFO] Claude output: %s", outputFile)

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")
	data, err := cmd.CombinedOutput()

	if data != nil {
		if writeErr := os.WriteFile(outputFile, data, 0644); writeErr != nil {
			log.Printf("[ERROR] Failed to write Claude output to %s: %v", outputFile, writeErr)
		}
	}

	if err != nil {
		return "", errors.Join(fmt.Errorf("Claude failed"), err)
	}

	log.Printf("[INFO] Claude completed the task")

	var claudeOutput Output
	if err := json.Unmarshal(data, &claudeOutput); err != nil {
		return "", err
	}

	return claudeOutput.Result, nil
}

func ParseStatus(output string) Status {
	normalized := strings.TrimSpace(strings.ToLower(output))

	switch {
	case strings.Contains(normalized, "running"):
		return TaskStatusRunning
	case strings.Contains(normalized, "finished"):
		return TaskStatusFinished
	default:
		return TaskStatusUnknown
	}
}
