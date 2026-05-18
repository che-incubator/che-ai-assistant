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

	"github.com/tolusha/che-doc-generator/pkg/github"
)

type OkPRReviewHandler struct{}

func NewOkPRReviewHandler() *OkPRReviewHandler {
	return &OkPRReviewHandler{}
}

func (g *OkPRReviewHandler) OnSuccess(
	ctx context.Context,
	result string,
	trigger *github.Trigger,
	gitHubClient *github.Client,
) {
}
