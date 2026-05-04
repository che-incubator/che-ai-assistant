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
	defaultPollInterval  = "5m"
	defaultTaskTimeout   = "30m"
	defaultMaxConcurrent = 1
)

var (
	defaultLogFile      = path.Join(os.TempDir(), "che-ai-assistant.log")
	defaultTemplatesDir = "templates"
)

type Config struct {
	WatchRepos    []string
	AllowedUsers  []string
	PollInterval  time.Duration
	TaskTimeout   time.Duration
	MaxConcurrent int
	TemplatesDir  string
	LogFile       string
}

func Parse() (*Config, error) {
	reposStr, err := requireEnv("CHE_AI_ASSISTANT_WATCH_REPOS")
	if err != nil {
		return nil, err
	}

	allowedUsersStr, err := requireEnv("CHE_AI_ASSISTANT_ALLOWED_USERS")
	if err != nil {
		return nil, err
	}

	templatesDir := optionalEnv("CHE_AI_ASSISTANT_TEMPLATES_DIR", defaultTemplatesDir)

	logFile := optionalEnv("CHE_AI_ASSISTANT_LOG_FILE", defaultLogFile)

	pollInterval, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_POLL_INTERVAL", defaultPollInterval))
	if err != nil {
		return nil, err
	}

	taskTimeout, err := parseDuration(optionalEnv("CHE_AI_ASSISTANT_TASK_TIMEOUT", defaultTaskTimeout))
	if err != nil {
		return nil, err
	}

	maxConcurrent := defaultMaxConcurrent
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

	return &Config{
		WatchRepos:    splitCSV(reposStr),
		PollInterval:  pollInterval,
		TaskTimeout:   taskTimeout,
		MaxConcurrent: maxConcurrent,
		TemplatesDir:  templatesDir,
		AllowedUsers:  splitCSV(allowedUsersStr),
		LogFile:       logFile,
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
