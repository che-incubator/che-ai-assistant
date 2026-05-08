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

package commands

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type SubCommandType string

const (
	Command = "/che-ai-assistant"

	WelcomeMarker = "<!-- che-ai-assistant-welcome -->"

	SubCommandGenerateCheDoc    SubCommandType = "generate-che-doc"
	SubCommandPullRequestReview SubCommandType = "ok-pr-review"
	SubCommandHelp              SubCommandType = "help"
)

type SubCommand struct {
	Type        SubCommandType
	Description string
}

var (
	parsePattern = regexp.MustCompile(`(?:^|\s)` + regexp.QuoteMeta(Command) + `(?:[ \t]+(\S+))?(?:[ \t]*$|[ \t]*\n|[ \t]+)`)

	SubCommands = []SubCommand{
		{Type: SubCommandGenerateCheDoc, Description: "Generate a documentation PR based on this PR's changes"},
		{Type: SubCommandPullRequestReview, Description: "Run a comprehensive PR review (summary, code review, deep review, impact analysis)"},
		{Type: SubCommandHelp, Description: "Show this help message"},
	}
)

func BuildWelcomeMessage(pollInterval time.Duration) string {
	var b strings.Builder

	b.WriteString(WelcomeMarker)
	b.WriteString("\n")
	b.WriteString("Hi! I'm **che-ai-assistant** — I help with your pull requests.\n\n")
	b.WriteString(fmt.Sprintf("I check for new commands every **%s** (if I am not busy :) ).\n\n", pollInterval))
	b.WriteString("**Available commands**:\n")

	for _, subCommand := range SubCommands {
		b.WriteString("- `" + Command + " " + string(subCommand.Type) + "` — " + subCommand.Description + "\n")
	}

	return b.String()
}

// Parse extracts a subcommand from a comment body containing the trigger prefix.
// Bare command with no subcommand returns SubCommandHelp.
// Returns false if the command prefix is not found.
func Parse(body string) (bool, SubCommandType) {
	items := parsePattern.FindStringSubmatch(body)
	if items == nil {
		return false, ""
	}

	sub := strings.TrimSpace(items[1])
	if sub == "" {
		return true, SubCommandHelp
	}

	return true, SubCommandType(sub)
}
