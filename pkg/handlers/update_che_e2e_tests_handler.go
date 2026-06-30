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
	"regexp"

	"github.com/tolusha/che-doc-generator/pkg/github"
)

var (
	updateCheE2ETestsPRURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che/pull/\d+`)
)

type UpdateCheE2ETestsHandler struct{}

func NewUpdateCheE2ETestsHandler() *UpdateCheE2ETestsHandler {
	return &UpdateCheE2ETestsHandler{}
}

func (g *UpdateCheE2ETestsHandler) OnError(
	ctx context.Context,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
}

func (g *UpdateCheE2ETestsHandler) OnSuccess(
	ctx context.Context,
	result string,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
	var body string

	prUrl, err := parseUpdateCheE2ETestsPRURL(result)
	if err != nil {
		log.Printf("[WARN] failed to parse E2E PR URL for %s/%s#%d: %v", trigger.Owner, trigger.Repo, trigger.PRNumber, err)
		body = fmt.Sprintf("%s\n\nFailed to extract E2E test fix PR URL from output.", trigger.CommentBody)
	} else {
		body = fmt.Sprintf("%s\n\nCreated E2E test fix PR: %s", trigger.CommentBody, prUrl)
	}

	if err := ghClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		body,
	); err != nil {
		log.Printf(
			"[ERROR] Failed to post on %s/%s#%d",
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
		)
	}
}

func parseUpdateCheE2ETestsPRURL(output string) (string, error) {
	match := updateCheE2ETestsPRURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}

	return match, nil
}
