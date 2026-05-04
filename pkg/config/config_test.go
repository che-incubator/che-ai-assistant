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
				"CHE_DOC_GENERATOR_WATCH_REPOS":        "org/repo1, org/repo2 ",
				"CHE_DOC_GENERATOR_ALLOWED_USERS":      "alice,bob",
				"CHE_DOC_GENERATOR_POLL_INTERVAL":      "5m",
				"CHE_DOC_GENERATOR_GENERATION_TIMEOUT": "1h",
				"CHE_DOC_GENERATOR_MAX_CONCURRENT":     "3",
				"CHE_DOC_GENERATOR_PROMPT_TEMPLATE":    "/custom/prompt.tmpl",
				"CHE_DOC_GENERATOR_LOG_FILE":           "/var/log/gen.log",
			},
			assertCfg: func(t *testing.T, cfg *Config) {
				assert.Equal(t, []string{"org/repo1", "org/repo2"}, cfg.WatchRepos)
				assert.Equal(t, []string{"alice", "bob"}, cfg.AllowedUsers)
				assert.Equal(t, 5*time.Minute, cfg.PollInterval)
				assert.Equal(t, 1*time.Hour, cfg.GenerationTimeout)
				assert.Equal(t, 3, cfg.MaxConcurrent)
				assert.Equal(t, "/custom/prompt.tmpl", cfg.PromptTemplatePath)
				assert.Equal(t, "/var/log/gen.log", cfg.LogFile)
			},
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", i), func(t *testing.T) {
			for key, val := range testCase.env {
				t.Setenv(key, val)
			}

			cfg, err := Parse()

			assert.NoError(t, err)
			testCase.assertCfg(t, cfg)
		})
	}
}
