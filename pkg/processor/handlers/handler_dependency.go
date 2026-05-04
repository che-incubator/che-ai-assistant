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
	"time"

	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/github"
)

type HandlerDependency struct {
	GHClient     *github.Client
	Timeout      time.Duration
	PollInterval time.Duration
	BuildPrompt  func(commands.SubCommandType, string) (string, error)
}
