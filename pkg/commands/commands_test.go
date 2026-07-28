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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		body                   string
		isOk                   bool
		expectedSubCommandType SubCommandType
		expectedArgs           string
	}{
		{"blabla /che-ai-assistant generate-che-doc", false, "", ""},
		{"blabla\n /che-ai-assistant generate-che-doc", false, "", ""},
		{"/che-ai-assistant generate-che-doc", true, SubCommandGenerateCheDoc, ""},
		{"/che-ai-assistant generate-che-doc\nsome text", true, SubCommandGenerateCheDoc, "some text"},
		{"/che-ai-assistant help", true, SubCommandHelp, ""},
		{"please /che-ai-assistant help thanks", false, "", ""},
		{"/che-ai-assistant check-pr-test-failures", true, SubCommandCheckPRTestFailures, ""},
		{"/che-ai-assistant   generate-che-doc    ", true, SubCommandGenerateCheDoc, ""},
		{"\n   /che-ai-assistant generate-che-doc", true, SubCommandGenerateCheDoc, ""},
		{"/che-ai-assistant", true, SubCommandHelp, ""},
		{"/che-ai-assistant\n", true, SubCommandHelp, ""},
		{"/che-ai-assistant  \n", true, SubCommandHelp, ""},
		{"just a regular comment", false, "", ""},
		{"/che-ai-assistantly", false, "", ""},
		{"/generate-che-doc", false, "", ""},
		{"/che-ai-assistant claude fix the typo in README", true, SubCommandClaude, "fix the typo in README"},
		{"/che-ai-assistant claude   spaces around args   ", true, SubCommandClaude, "spaces around args"},
		{"/che-ai-assistant claude multiline\ninstruction here", true, SubCommandClaude, "multiline\ninstruction here"},
		{"/che-ai-assistant claude", true, SubCommandClaude, ""},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprintf("Case #%d", i), func(t *testing.T) {
			ok, subCommandType, args := Parse(test.body)

			assert.Equal(t, test.isOk, ok)
			assert.Equal(t, test.expectedSubCommandType, subCommandType)
			assert.Equal(t, test.expectedArgs, args)
		})
	}
}

func TestBuildWarningMessage(t *testing.T) {
	files := []string{".claude/settings.json", ".vscode/launch.json"}
	msg := BuildWarningMessage(files)

	assert.Contains(t, msg, WarningMarker)
	assert.Contains(t, msg, ".claude/settings.json")
	assert.Contains(t, msg, ".vscode/launch.json")
}

func TestBuildWarningMessage_SingleFile(t *testing.T) {
	files := []string{".idea/workspace.xml"}
	msg := BuildWarningMessage(files)

	assert.Contains(t, msg, WarningMarker)
	assert.Contains(t, msg, ".idea/workspace.xml")
}

func TestIsCommandAvailableForRepo(t *testing.T) {
	tests := []struct {
		name     string
		sub      SubCommandType
		repo     string
		expected bool
	}{
		{
			name:     "unrestricted command available for any repo",
			sub:      SubCommandGenerateCheDoc,
			repo:     "some-org/some-repo",
			expected: true,
		},
		{
			name:     "restricted command available for allowed repo",
			sub:      SubCommandPullRequestReadiness,
			repo:     "devfile/devworkspace-operator",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommandAvailableForRepo(tt.sub, tt.repo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildWelcomeMessage_ShowsAllCommandsForUnrestrictedRepo(t *testing.T) {
	msg := BuildPRWelcomeMessage("devfile/devworkspace-operator", 5*time.Minute)

	assert.Contains(t, msg, string(SubCommandGenerateCheDoc))
	assert.Contains(t, msg, string(SubCommandPullRequestReview))
	assert.Contains(t, msg, string(SubCommandHelp))
	assert.Contains(t, msg, string(SubCommandPullRequestReadiness))
	assert.Contains(t, msg, string(SubCommandCheckPRTestFailures))
}
