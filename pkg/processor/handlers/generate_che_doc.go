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
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"

	"github.com/tolusha/che-doc-generator/pkg/github"
)

var (
	prURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che-docs/pull/\d+`)
)

func HandleGenerateCheDoc(ctx context.Context, trigger *github.Trigger, deps *HandlerDependency) {
	docPR, err := generate(ctx, trigger, deps)
	if err != nil {
		updateCommentWithFailureMessage(ctx, trigger, deps, err)
		return
	}

	updateCommentWithDocPR(ctx, trigger, deps, docPR)
}

func generate(ctx context.Context, trigger *github.Trigger, deps *HandlerDependency) (string, error) {
	prompt, err := deps.BuildPrompt(trigger.SubCommand, trigger.PRURL)
	if err != nil {
		return "", err
	}

	log.Printf("[INFO] claude prompt:\n%s", prompt)

	ctx, cancel := context.WithTimeout(ctx, deps.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("timed out after %v", deps.Timeout)
	}

	if err != nil {
		return "", fmt.Errorf("claude exited with error: %w\noutput: %s", err, string(output))
	}

	log.Printf("[INFO] claude output:\n%s", string(output))

	return parseDocPRURL(string(output))
}

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}

	return match, nil
}

func updateCommentWithFailureMessage(
	ctx context.Context,
	trigger *github.Trigger,
	deps *HandlerDependency,
	err error,
) {
	log.Printf("[ERROR] %s failed for %s/%s#%d: %v", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber, err)

	msg := fmt.Sprintf("%s\n\nFailed to generate documentation.", trigger.CommentBody)
	if err := deps.GHClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		msg,
	); err != nil {
		log.Printf("[ERROR] error posting failure comment: %v", err)
	}
}

func updateCommentWithDocPR(
	ctx context.Context,
	trigger *github.Trigger,
	deps *HandlerDependency,
	docPR string,
) {
	log.Printf("[INFO] %s completed for %s/%s#%d: %s", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber, docPR)

	msg := fmt.Sprintf("%s\n\nDocumentation PR created: %s", trigger.CommentBody, docPR)
	if err := deps.GHClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		msg,
	); err != nil {
		log.Printf("[ERROR] error posting success comment: %v", err)
	}
}
