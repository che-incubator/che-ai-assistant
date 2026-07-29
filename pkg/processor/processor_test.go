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

package processor

import (
	"che-incubator/che-ai-assistant/pkg/commands"
	"che-incubator/che-ai-assistant/pkg/github"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevWorkspaceName(t *testing.T) {
	tests := []struct {
		name     string
		trigger  *github.Trigger
		expected string
	}{
		{
			name: "basic name",
			trigger: &github.Trigger{
				SubCommandType: commands.SubCommandPullRequestReview,
				Repo:           "my-repo",
				IssueNumber:    42,
			},
			expected: "che-ai-ok-pr-review-my-repo-42",
		},
		{
			name: "uppercase converted to lowercase",
			trigger: &github.Trigger{
				SubCommandType: commands.SubCommandPullRequestReview,
				Repo:           "My-Repo",
				IssueNumber:    1,
			},
			expected: "che-ai-ok-pr-review-my-repo-1",
		},
		{
			name: "dots replaced with underscores",
			trigger: &github.Trigger{
				SubCommandType: commands.SubCommandClaude,
				Repo:           "my.dotted.repo",
				IssueNumber:    7,
			},
			expected: "che-ai-claude-my_dotted_repo-7",
		},
		{
			name: "name truncated to 63 characters",
			trigger: &github.Trigger{
				SubCommandType: commands.SubCommandCheckPRTestFailures,
				Repo:           "a-very-long-repository-name-that-will-exceed-the-limit",
				IssueNumber:    12345,
			},
			expected: func() string {
				result := getDevWorkspaceName(&github.Trigger{
					SubCommandType: commands.SubCommandCheckPRTestFailures,
					Repo:           "a-very-long-repository-name-that-will-exceed-the-limit",
					IssueNumber:    12345,
				})
				assert.LessOrEqual(t, len(result), 63)
				assert.True(t, strings.HasSuffix(result, "-12345"))
				return result
			}(),
		},
		{
			name: "truncation does not leave trailing dash",
			trigger: &github.Trigger{
				SubCommandType: commands.SubCommandCheckPRTestFailures,
				Repo:           "a-very-long-repository-name-that-will-definitely-exceed",
				IssueNumber:    99,
			},
			expected: func() string {
				result := getDevWorkspaceName(&github.Trigger{
					SubCommandType: commands.SubCommandCheckPRTestFailures,
					Repo:           "a-very-long-repository-name-that-will-definitely-exceed",
					IssueNumber:    99,
				})
				assert.LessOrEqual(t, len(result), 63)
				assert.True(t, strings.HasSuffix(result, "-99"))
				prefix := strings.TrimSuffix(result, "-99")
				assert.False(t, strings.HasSuffix(prefix, "-"))
				return result
			}(),
		},
		{
			name: "name exactly at 63 characters is not truncated",
			trigger: &github.Trigger{
				SubCommandType: "cmd",
				Repo:           strings.Repeat("a", 63-len("che-ai-cmd--1")),
				IssueNumber:    1,
			},
			expected: "che-ai-cmd-" + strings.Repeat("a", 63-len("che-ai-cmd--1")) + "-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDevWorkspaceName(tt.trigger)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), 63)
		})
	}
}
