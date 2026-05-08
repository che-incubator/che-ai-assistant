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
	"fmt"
	"log"
	"os/exec"
	"regexp"

	"github.com/tolusha/che-doc-generator/pkg/github"
)

var (
	prURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che-docs/compare/[^\s"]+`)
)

type GenerateCheDocHandler struct {
}

func NewGenerateCheDocHandler() *GenerateCheDocHandler {
	return &GenerateCheDocHandler{}
}

func (g *GenerateCheDocHandler) Run(
	ctx context.Context,
	prompt string,
	trigger *github.Trigger,
	ghClient *github.Client,
) (string, error) {
	log.Printf("[INFO] Running claude >>>>>>>>")
	log.Printf("[INFO] Prompt:\n%s", prompt)
	log.Printf("[INFO] Running claude <<<<<<<<")

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")
	rawData, err := cmd.CombinedOutput()

	output := string(rawData)

	if err != nil {
		return output, err
	}

	prUrl, err := parseDocPRURL(output)
	if err != nil {
		return output, fmt.Errorf("error parsing doc PR URL: %s", err)
	}

	return prUrl, nil
}

func (g *GenerateCheDocHandler) OnFailure(
	ctx context.Context,
	output string,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
	body := fmt.Sprintf("%s\n\nFailed to generate documentation.", trigger.CommentBody)
	updatePullRequestComment(ctx, body, trigger, ghClient)
}

func (g *GenerateCheDocHandler) OnSuccess(
	ctx context.Context,
	output string,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
	body := fmt.Sprintf("%s\n\nCreated documentation PR: %s", trigger.CommentBody, output)
	updatePullRequestComment(ctx, body, trigger, ghClient)
}

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no compare URL found in output")
	}

	return match, nil
}
