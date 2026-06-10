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
	"regexp"
	"strings"
)

type SubCommandType string

const (
	Command = "/che-ai-assistant"

	WelcomeMarker = "<!-- che-ai-assistant-welcome -->"
	WarningMarker = "<!-- che-ai-assistant:file-warning -->"

	SubCommandGenerateCheDoc    SubCommandType = "generate-che-doc"
	SubCommandPullRequestReview SubCommandType = "ok-pr-review"
	SubCommandHelp              SubCommandType = "help"
)

type SubCommand struct {
	Type        SubCommandType
	Description string
}

var (
	parsePattern = regexp.MustCompile(`^\s*` + regexp.QuoteMeta(Command) + `(?:\s+(\S+))?(?:\s|$)`)

	SubCommands = []SubCommand{
		{Type: SubCommandGenerateCheDoc, Description: "Generate a documentation PR based on this PR's changes"},
		{Type: SubCommandPullRequestReview, Description: "Run a comprehensive PR review (summary, code review, deep review, impact analysis)"},
		{Type: SubCommandHelp, Description: "Show this help message"},
	}
)

func BuildWelcomeMessage() string {
	var b strings.Builder

	b.WriteString(WelcomeMarker)
	b.WriteString("\n")
	b.WriteString("Hi! I'm **che-ai-assistant** — I help with your pull requests.\n\n")
	b.WriteString("**Available commands**:\n")

	for _, subCommand := range SubCommands {
		b.WriteString("- `" + Command + " " + string(subCommand.Type) + "` — " + subCommand.Description + "\n")
	}

	return b.String()
}

func BuildWarningMessage(files []string) string {
	var b strings.Builder

	b.WriteString(WarningMarker)
	b.WriteString("\n")
	b.WriteString("⚠️ **Warning: IDE/tool configuration files detected**\n\n")
	b.WriteString("This PR contains changes to files in directories that are typically not intended to be committed:\n\n")

	for _, f := range files {
		b.WriteString("- `" + f + "`\n")
	}

	b.WriteString("\nPlease verify these changes are intentional.\n")

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
