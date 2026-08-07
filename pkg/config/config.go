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
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPollInterval               = "10m"
	defaultTaskTimeout                = "30m"
	defaultImplementTaskTimeout       = "12h"
	defaultMaxConcurrentTasks         = 1
	defaultPromptsDir                 = "./prompts"
	defaultWarnDirs                   = ".claude,.vscode"
	defaultLogFile                    = "./che-ai-assistant.log"
	defaultSkillRepositoryURL         = "https://github.com/che-incubator/che-ai-assistant-skills"
	defaultSkillRepositoryBranch      = "main"
	defaultMCPServerURL               = "http://che-mcp-server:8080/mcp"
	defaultSupervisorRepositoryUrl    = "https://github.com/akurinnoy/supervisor-terminal"
	defaultSupervisorRepositoryBranch = "main"
)

type Config struct {
	GitHubRepositories         []string
	GitHubUsers                []string
	GitHubToken                string
	GitHubPollInterval         time.Duration
	TaskTimeout                time.Duration
	ImplementTaskTimeout       time.Duration
	MaxConcurrentTasks         int
	PromptsDir                 string
	LogFile                    string
	MCPServerURL               string
	WarnDirsCommits            []string
	StateFile                  string
	SkillsRepositoryURL        string
	SkillsRepositoryBranch     string
	SupervisorRepositoryUrl    string
	SupervisorRepositoryBranch string
}

func Read() (*Config, error) {
	githubToken, err := requireEnv("CHE_AI_ASSISTANT_GITHUB_TOKEN")
	if err != nil {
		return nil, err
	}

	githubRepositoriesStr, err := requireEnv("CHE_AI_ASSISTANT_GITHUB_REPOSITORIES")
	if err != nil {
		return nil, err
	}

	githubUsersStr, err := requireEnv("CHE_AI_ASSISTANT_GITHUB_USERS")
	if err != nil {
		return nil, err
	}

	promptsDir := optionalEnv("CHE_AI_ASSISTANT_PROMPTS_DIR", defaultPromptsDir)

	logFile := optionalEnv("CHE_AI_ASSISTANT_LOG_FILE", defaultLogFile)

	githubPollInterval, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_GITHUB_POLL_INTERVAL", defaultPollInterval))
	if err != nil {
		return nil, err
	}

	taskTimeout, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_TASK_TIMEOUT", defaultTaskTimeout))
	if err != nil {
		return nil, err
	}

	implementTaskTimeout, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_IMPLEMENT_TASK_TIMEOUT", defaultImplementTaskTimeout))
	if err != nil {
		return nil, err
	}

	maxConcurrentTasks := defaultMaxConcurrentTasks
	if v := os.Getenv("CHE_AI_ASSISTANT_MAX_CONCURRENT_TASKS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CHE_AI_ASSISTANT_MAX_CONCURRENT_TASKS: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("CHE_AI_ASSISTANT_MAX_CONCURRENT_TASKS must be positive, got %d", n)
		}
		maxConcurrentTasks = n
	}

	mcpServerURL := optionalEnv("CHE_AI_ASSISTANT_MCP_SERVER_URL", defaultMCPServerURL)

	skillsRepositoryURL := optionalEnv("CHE_AI_ASSISTANT_TASKS_SKILLS_REPOSITORY_URL", defaultSkillRepositoryURL)
	skillsRepositoryBranch := optionalEnv("CHE_AI_ASSISTANT_TASKS_SKILLS_REPOSITORY_BRANCH", defaultSkillRepositoryBranch)

	supervisorRepositoryUrl := optionalEnv("CHE_AI_ASSISTANT_TASKS_SUPERVISOR_REPOSITORY_URL", defaultSupervisorRepositoryUrl)
	supervisorRepositoryBranch := optionalEnv("CHE_AI_ASSISTANT_TASKS_SUPERVISOR_REPOSITORY_BRANCH", defaultSupervisorRepositoryBranch)

	warnDirsCommits := splitCSV(optionalEnv("CHE_AI_ASSISTANT_WARN_DIRS_COMMITS", defaultWarnDirs))

	stateFile := os.Getenv("CHE_AI_ASSISTANT_STATE_FILE")
	if stateFile == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting user home directory: %w", err)
		}
		stateFile = path.Join(userHome, "state.json")
	}

	return &Config{
		GitHubRepositories:         splitCSV(githubRepositoriesStr),
		GitHubPollInterval:         githubPollInterval,
		TaskTimeout:                taskTimeout,
		ImplementTaskTimeout:       implementTaskTimeout,
		MaxConcurrentTasks:         maxConcurrentTasks,
		PromptsDir:                 promptsDir,
		GitHubUsers:                splitCSV(githubUsersStr),
		LogFile:                    logFile,
		MCPServerURL:               mcpServerURL,
		GitHubToken:                githubToken,
		WarnDirsCommits:            warnDirsCommits,
		StateFile:                  stateFile,
		SkillsRepositoryURL:        skillsRepositoryURL,
		SkillsRepositoryBranch:     skillsRepositoryBranch,
		SupervisorRepositoryUrl:    supervisorRepositoryUrl,
		SupervisorRepositoryBranch: supervisorRepositoryBranch,
	}, nil
}

func splitCSV(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

func requireEnv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("%s environment variable is required", name)
	}

	return v, nil
}

func optionalEnv(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}

	return value
}

func parseDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", value, err)
	}

	return duration, nil
}
