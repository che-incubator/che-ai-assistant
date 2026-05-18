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
	prURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che-docs/pull/\d+`)
)

type GenerateCheDocHandler struct{}

func NewGenerateCheDocHandler() *GenerateCheDocHandler {
	return &GenerateCheDocHandler{}
}

func (g *GenerateCheDocHandler) OnError(
	ctx context.Context,
	trigger *github.Trigger,
	gitHubClient *github.Client) {
}

func (g *GenerateCheDocHandler) OnSuccess(
	ctx context.Context,
	result string,
	trigger *github.Trigger,
	gitHubClient *github.Client,
) {
	var body string

	prUrl, err := parseDocPRURL(result)
	if err != nil {
		body = fmt.Sprintf("%s\n\nDocumentation PR not found.", trigger.CommentBody)
	} else {
		body = fmt.Sprintf("%s\n\nCreated documentation PR: %s", trigger.CommentBody, prUrl)
	}

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

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}

	return match, nil
}
