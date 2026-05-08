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

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/tolusha/che-doc-generator/pkg/github"
)

const outputDelimiter = "===OK-PR-REVIEW-OUTPUT==="

type claudeOutput struct {
	Result string `json:"result"`
}

type OkPRReviewHandler struct{}

func NewOkPRReviewHandler() *OkPRReviewHandler {
	return &OkPRReviewHandler{}
}

func (g *OkPRReviewHandler) Run(
	ctx context.Context,
	prompt string,
	trigger *github.Trigger,
	ghClient *github.Client,
) ([]string, error) {
	log.Printf("[INFO] Running claude >>>>>>>>")
	log.Printf("[INFO] Prompt:\n%s", prompt)
	log.Printf("[INFO] Running claude <<<<<<<<")

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")
	rawData, err := cmd.CombinedOutput()

	if err != nil {
		return []string{string(rawData)}, err
	}

	var parsed claudeOutput
	if err := json.Unmarshal(rawData, &parsed); err != nil {
		return []string{string(rawData)}, fmt.Errorf("failed to parse claude JSON output: %w", err)
	}

	parts := strings.Split(parsed.Result, outputDelimiter)

	var outputs []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			outputs = append(outputs, trimmed)
		}
	}

	if len(outputs) == 0 {
		return []string{parsed.Result}, fmt.Errorf("no review outputs found in delimited result")
	}

	return outputs, nil
}

func (g *OkPRReviewHandler) OnFailure(
	ctx context.Context,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
	body := fmt.Sprintf("%s\n\nFailed to review the PR.", trigger.CommentBody)

	err := ghClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		body,
	)

	if err != nil {
		log.Printf(
			"[ERROR] Failed to post on %s/%s#%d",
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
		)
	}
}

func (g *OkPRReviewHandler) OnSuccess(
	ctx context.Context,
	outputs []string,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
	for _, output := range outputs {
		err := ghClient.PostPullRequestComment(
			ctx,
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
			output,
		)
		if err != nil {
			log.Printf(
				"[ERROR] Failed to post on %s/%s#%d",
				trigger.Owner,
				trigger.Repo,
				trigger.PRNumber,
			)
		}
	}
}
