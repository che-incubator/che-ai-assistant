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

	"github.com/tolusha/che-doc-generator/pkg/github"
)

type CheckPRTestFailuresHandler struct{}

func NewCheckPRTestFailuresHandler() *CheckPRTestFailuresHandler {
	return &CheckPRTestFailuresHandler{}
}

func (g *CheckPRTestFailuresHandler) OnError(
	ctx context.Context,
	trigger *github.Trigger,
	gitHubClient *github.Client) {
}

func (g *CheckPRTestFailuresHandler) OnSuccess(
	ctx context.Context,
	result string,
	trigger *github.Trigger,
	gitHubClient *github.Client,
) {
	body := fmt.Sprintf("%s\n\nReview is complete. Please check the review comments below.", trigger.CommentBody)

	if err := gitHubClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		body,
	); err != nil {
		log.Printf(
			"[ERROR] Failed to post on %s/%s#%d: %v",
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
			err,
		)
	}
}
