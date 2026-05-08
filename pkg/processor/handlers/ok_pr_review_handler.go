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
	"strings"

	"github.com/tolusha/che-doc-generator/pkg/github"
)

const outputDelimiter = "===OK-PR-REVIEW-OUTPUT==="

type OkPRReviewHandler struct{}

func NewOkPRReviewHandler() *OkPRReviewHandler {
	return &OkPRReviewHandler{}
}

func (g *OkPRReviewHandler) OnError(
	ctx context.Context,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
}

func (g *OkPRReviewHandler) OnSuccess(
	ctx context.Context,
	result string,
	trigger *github.Trigger,
	ghClient *github.Client,
) {
	parts := strings.Split(result, outputDelimiter)

	var reviews []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			reviews = append(reviews, trimmed)
		}
	}

	if len(reviews) < 4 {
		body := fmt.Sprintf("%s\n\nPullRequest review not found.", trigger.CommentBody)

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

		return
	}

	for _, output := range reviews {
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
