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

package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		env       map[string]string
		assertCfg func(t *testing.T, cfg *Config)
	}{
		{
			env: map[string]string{
				"CHE_AI_ASSISTANT_GITHUB_REPOSITORIES":  "org/repo1, org/repo2 ",
				"CHE_AI_ASSISTANT_GITHUB_USERS":         "alice,bob",
				"CHE_AI_ASSISTANT_GITHUB_TOKEN":         "token",
				"CHE_AI_ASSISTANT_GITHUB_POLL_INTERVAL": "5m",
				"CHE_AI_ASSISTANT_TASK_TIMEOUT":         "1h",
				"CHE_AI_ASSISTANT_MAX_CONCURRENT_TASKS": "3",
				"CHE_AI_ASSISTANT_PROMPTS_DIR":          "/custom/prompts",
				"CHE_AI_ASSISTANT_LOG_FILE":             "/var/log/gen.log",
				"CHE_AI_ASSISTANT_MCP_SERVER_URL":       "http://che-mcp-server:8080/mcp",
			},
			assertCfg: func(t *testing.T, cfg *Config) {
				assert.Equal(t, []string{"org/repo1", "org/repo2"}, cfg.GitHubRepositories)
				assert.Equal(t, []string{"alice", "bob"}, cfg.GitHubUsers)
				assert.Equal(t, 5*time.Minute, cfg.GitHubPollInterval)
				assert.Equal(t, 1*time.Hour, cfg.TaskTimeout)
				assert.Equal(t, 3, cfg.MaxConcurrentTasks)
				assert.Equal(t, "/custom/prompts", cfg.PromptsDir)
				assert.Equal(t, "/var/log/gen.log", cfg.LogFile)
				assert.Equal(t, "token", cfg.GitHubToken)
				assert.Equal(t, "http://che-mcp-server:8080/mcp", cfg.MCPServerURL)
			},
		},
		{
			env: map[string]string{
				"CHE_AI_ASSISTANT_GITHUB_REPOSITORIES": "org/repo1",
				"CHE_AI_ASSISTANT_GITHUB_USERS":        "alice",
				"CHE_AI_ASSISTANT_GITHUB_TOKEN":        "token",
				"CHE_AI_ASSISTANT_MCP_SERVER_URL":      "http://mcp:8080",
				"CHE_AI_ASSISTANT_WARN_DIRS_COMMITS":   ".claude,.vscode,.idea",
			},
			assertCfg: func(t *testing.T, cfg *Config) {
				assert.Equal(t, []string{".claude", ".vscode", ".idea"}, cfg.WarnDirsCommits)
			},
		},
		{
			env: map[string]string{
				"CHE_AI_ASSISTANT_GITHUB_REPOSITORIES": "org/repo1",
				"CHE_AI_ASSISTANT_GITHUB_USERS":        "alice",
				"CHE_AI_ASSISTANT_GITHUB_TOKEN":        "token",
				"CHE_AI_ASSISTANT_MCP_SERVER_URL":      "http://mcp:8080",
			},
			assertCfg: func(t *testing.T, cfg *Config) {
				assert.Equal(t, []string{".claude", ".vscode"}, cfg.WarnDirsCommits)
			},
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", i), func(t *testing.T) {
			for key, val := range testCase.env {
				t.Setenv(key, val)
			}

			cfg, err := Read()

			assert.NoError(t, err)
			testCase.assertCfg(t, cfg)
		})
	}
}
