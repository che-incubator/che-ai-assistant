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
	StatusRunning  Status = "Running"
	StatusFinished Status = "Finished"
	StatusLost     Status = "Lost"
	StatusIdle     Status = "Idle"
	StatusUnknown  Status = "Unknown"
)

func New(cfg *config.Config) *Claude {
	return &Claude{outputDir: cfg.ClaudeOutputDir}
}

func (r *Claude) Run(ctx context.Context, timeout time.Duration, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	outputFile := filepath.Join(r.outputDir, fmt.Sprintf("claude-output-%d.json", time.Now().UnixNano()))

	log.Printf("[INFO] Claude started, output file %s", outputFile)

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")
	data, err := cmd.CombinedOutput()

	if data != nil {
		f, fileErr := os.OpenFile(outputFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if fileErr != nil {
			return "", errors.Join(fmt.Errorf("failed to open file %s", outputFile), fileErr)
		}
		defer func() {
			_ = f.Close()
		}()

		_, fileErr = f.Write(data)
		if fileErr != nil {
			return "", errors.Join(fmt.Errorf("failed to write into %s", outputFile), fileErr)
		}
	}

	if err != nil {
		return "", errors.Join(fmt.Errorf("Claude failed the task"), err)
	}

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
		return StatusRunning
	case strings.Contains(normalized, "finished"):
		return StatusFinished
	case strings.Contains(normalized, "lost"):
		return StatusLost
	case strings.Contains(normalized, "idle"):
		return StatusIdle
	default:
		return StatusUnknown
	}
}
