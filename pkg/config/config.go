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
	defaultPollInterval       = "5m"
	defaultTaskTimeout        = "30m"
	defaultMaxConcurrentTasks = 1
	defaultTemplatesDir       = "templates"
	defaultDeleteDevWorkspace = "true"
	defaultWarnDirs           = ".claude,.vscode"
)

var (
	defaultLogFile   = path.Join(os.TempDir(), "che-ai-pullrequest-assistant.log")
	defaultOutputDir = os.TempDir()
)

type Config struct {
	GitHubWatchRepos   []string
	GitHubAllowedUsers []string
	GitHubToken        string
	TasksPollInterval  time.Duration
	TaskTimeout        time.Duration
	MaxConcurrentTasks int
	TemplatesDir       string
	OutputDir          string
	LogFile            string
	MCPServerURL       string
	DeleteDevWorkspace bool
	WarnDirsCommits    []string
}

func Read() (*Config, error) {
	githubToken, err := requireEnv("CHE_AI_ASSISTANT_GITHUB_TOKEN")
	if err != nil {
		return nil, err
	}

	githubWatchReposStr, err := requireEnv("CHE_AI_ASSISTANT_GITHUB_WATCH_REPOS")
	if err != nil {
		return nil, err
	}

	githubAllowedUsersStr, err := requireEnv("CHE_AI_ASSISTANT_GITHUB_ALLOWED_USERS")
	if err != nil {
		return nil, err
	}

	templatesDir := optionalEnv("CHE_AI_ASSISTANT_TEMPLATES_DIR", defaultTemplatesDir)

	outputDir := optionalEnv("CHE_AI_ASSISTANT_OUTPUT_DIR", defaultOutputDir)

	logFile := optionalEnv("CHE_AI_ASSISTANT_LOG_FILE", defaultLogFile)

	tasksPollInterval, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_POLL_INTERVAL", defaultPollInterval))
	if err != nil {
		return nil, err
	}

	taskTimeout, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_TASK_TIMEOUT", defaultTaskTimeout))
	if err != nil {
		return nil, err
	}

	maxConcurrent := defaultMaxConcurrentTasks
	if v := os.Getenv("CHE_AI_ASSISTANT_MAX_CONCURRENT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CHE_AI_ASSISTANT_MAX_CONCURRENT: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("CHE_AI_ASSISTANT_MAX_CONCURRENT must be positive, got %d", n)
		}
		maxConcurrent = n
	}

	mcpServerURL, err := requireEnv("CHE_AI_MCP_SERVER_URL")
	if err != nil {
		return nil, err
	}

	deleteDevWorkspace, err := strconv.ParseBool(optionalEnv("CHE_DELETE_DEV_WORKSPACE", defaultDeleteDevWorkspace))
	if err != nil {
		return nil, err
	}

	warnDirsCommits := splitCSV(optionalEnv("CHE_AI_ASSISTANT_WARN_DIRS_COMMITS", defaultWarnDirs))

	return &Config{
		GitHubWatchRepos:   splitCSV(githubWatchReposStr),
		TasksPollInterval:  tasksPollInterval,
		TaskTimeout:        taskTimeout,
		MaxConcurrentTasks: maxConcurrent,
		TemplatesDir:       templatesDir,
		OutputDir:          outputDir,
		GitHubAllowedUsers: splitCSV(githubAllowedUsersStr),
		LogFile:            logFile,
		MCPServerURL:       mcpServerURL,
		GitHubToken:        githubToken,
		DeleteDevWorkspace: deleteDevWorkspace,
		WarnDirsCommits:    warnDirsCommits,
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
