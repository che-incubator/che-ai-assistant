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

	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/github"
)

func HandleHelp(ctx context.Context, trigger *github.Trigger, deps *HandlerDependency) {
	if err := deps.GHClient.PostPullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		commands.BuildWelcomeMessage(deps.PollInterval),
	); err != nil {
		log.Printf("[ERROR] error posting help comment: %v", err)
	}
}
