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

// Handler defines the lifecycle for processing a bot subcommand triggered by a PR comment.
type Handler interface {
	// Run executes the subcommand logic and returns its raw output.
	Run(
		ctx context.Context,
		prompt string,
		trigger *github.Trigger,
		ghClient *github.Client,
	) ([]string, error)

	// OnFailure is called when Run returns an error, allowing the handler to update the PR comment.
	OnFailure(
		ctx context.Context,
		trigger *github.Trigger,
		ghClient *github.Client,
	)

	// OnSuccess is called when Run succeeds, allowing the handler to post results to the PR.
	OnSuccess(
		ctx context.Context,
		outputs []string,
		trigger *github.Trigger,
		ghClient *github.Client,
	)
}
