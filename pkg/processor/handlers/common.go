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

	"github.com/tolusha/che-doc-generator/pkg/github"
)

func updatePullRequestComment(ctx context.Context, body string, trigger *github.Trigger, ghClient *github.Client) {
	err := ghClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		body,
	)

	if err != nil {
		log.Println("[ERROR] Failed to update pull request comment:", err)
	}
}
