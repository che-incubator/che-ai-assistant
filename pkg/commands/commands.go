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
	"slices"
	"strings"
)

type SubCommandType string

const (
	Command = "/che-ai-assistant"

	WelcomeMarker = "<!-- che-ai-assistant-welcome -->"
	WarningMarker = "<!-- che-ai-assistant:file-warning -->"

	AutoTriggerMarkerPrefix = "<!-- che-ai-assistant:auto-trigger:"
	AutoTriggerMarkerFmt    = AutoTriggerMarkerPrefix + "%s -->"

	SubCommandGenerateCheDoc       SubCommandType = "generate-che-doc"
	SubCommandPullRequestReview    SubCommandType = "ok-pr-review"
	SubCommandPullRequestReadiness SubCommandType = "ok-pr-readiness"
	SubCommandCheckPRTestFailures  SubCommandType = "check-pr-test-failures"
	SubCommandHelp                 SubCommandType = "help"
)

type SubCommand struct {
	Type         SubCommandType
	Description  string
	AllowedRepos []string
	AutoTrigger  bool
}

var (
	parsePattern = regexp.MustCompile(`^\s*` + regexp.QuoteMeta(Command) + `(?:\s+(\S+))?(?:\s|$)`)

	SubCommands = []SubCommand{
		{
			Type:        SubCommandGenerateCheDoc,
			Description: "Generate a documentation PR based on this PR's changes",
		},
		{
			Type:        SubCommandPullRequestReview,
			Description: "Run a comprehensive PR review (summary, code review, deep review, impact analysis)",
		},
		{
			Type:         SubCommandPullRequestReadiness,
			Description:  "Ensure PR has validation steps",
			AllowedRepos: []string{"devfile/devworkspace-operator"},
			AutoTrigger:  true,
		},
		{
			Type:        SubCommandCheckPRTestFailures,
			Description: "Analyze failing CI checks, identify root causes, and suggest fixes",
		},
		{
			Type:        SubCommandHelp,
			Description: "Show this help message",
		},
	}
)

func BuildWelcomeMessage(repoFullName string) string {
	var b strings.Builder

	b.WriteString(WelcomeMarker)
	b.WriteString("\n")
	b.WriteString("Hi! I'm **che-ai-assistant** — I help with your pull requests.\n\n")
	b.WriteString("**Available commands**:\n")

	for _, subCommand := range SubCommands {
		if len(subCommand.AllowedRepos) > 0 && !slices.Contains(subCommand.AllowedRepos, repoFullName) {
			continue
		}
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

func AutoTriggerMarker(sub SubCommandType) string {
	return fmt.Sprintf(AutoTriggerMarkerFmt, sub)
}

func BuildAutoTriggerComment(sub SubCommandType) string {
	return fmt.Sprintf("%s %s\n%s", Command, sub, AutoTriggerMarker(sub))
}

func IsAutoTriggerComment(body string) bool {
	return strings.Contains(body, AutoTriggerMarkerPrefix)
}

// IsCommandAvailableForRepo returns true if the subcommand is available for the given repository.
// Commands with an empty AllowedRepos list are available for all repositories.
// Commands with a non-empty AllowedRepos list are only available for repositories in that list.
func IsCommandAvailableForRepo(sub SubCommandType, repoFullName string) bool {
	for _, sc := range SubCommands {
		if sc.Type == sub {
			return len(sc.AllowedRepos) == 0 || slices.Contains(sc.AllowedRepos, repoFullName)
		}
	}

	return false
}
