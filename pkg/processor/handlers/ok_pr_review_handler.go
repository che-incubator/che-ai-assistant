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

	if len(reviews) == 0 {
		log.Printf("[ERROR] No reviews found for PR %s", trigger.PRURL)
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
