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

package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tolusha/che-doc-generator/pkg/config"
)

func writeTemplate(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		expectErr string
	}{
		{
			name:     "valid template",
			template: "Generate docs for {{.PRURL}}",
		},
		{
			name:      "empty template",
			template:  "   ",
			expectErr: "is empty",
		},
		{
			name:      "missing PRURL placeholder",
			template:  "Generate docs for this PR",
			expectErr: "must contain {{.PRURL}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemplate(t, tt.template)
			cfg := &config.Config{PromptTemplatePath: path}

			g, err := New(nil, cfg)

			if tt.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErr)
				assert.Nil(t, g)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, g)
			}
		})
	}
}

func TestNew_MissingFile(t *testing.T) {
	cfg := &config.Config{PromptTemplatePath: "/nonexistent/prompt.tmpl"}

	g, err := New(nil, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading prompt template")
	assert.Nil(t, g)
}

func TestBuildPrompt(t *testing.T) {
	g := &Generator{template: "Generate docs for {{.PRURL}} please"}

	result, err := g.buildPrompt("https://github.com/org/repo/pull/42")

	require.NoError(t, err)
	assert.Equal(t, "Generate docs for https://github.com/org/repo/pull/42 please", result)
}

func TestBuildPrompt_MultiPlaceholder(t *testing.T) {
	g := &Generator{template: "PR: {{.PRURL}} — see {{.PRURL}}"}

	result, err := g.buildPrompt("https://github.com/org/repo/pull/1")

	require.NoError(t, err)
	assert.Equal(t, "PR: https://github.com/org/repo/pull/1 — see https://github.com/org/repo/pull/1", result)
}

func TestParseDocPRURL(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		expected  string
		expectErr bool
	}{
		{
			name:     "URL in output",
			output:   `Some text\nhttps://github.com/eclipse-che/che-docs/pull/123\nDone`,
			expected: "https://github.com/eclipse-che/che-docs/pull/123",
		},
		{
			name:     "URL embedded in JSON",
			output:   `{"result": "https://github.com/eclipse-che/che-docs/pull/456"}`,
			expected: "https://github.com/eclipse-che/che-docs/pull/456",
		},
		{
			name:     "multiple URLs returns first",
			output:   "https://github.com/eclipse-che/che-docs/pull/1 and https://github.com/eclipse-che/che-docs/pull/2",
			expected: "https://github.com/eclipse-che/che-docs/pull/1",
		},
		{
			name:      "no URL in output",
			output:    "No PR was created",
			expectErr: true,
		},
		{
			name:      "wrong repo URL",
			output:    "https://github.com/other-org/other-repo/pull/99",
			expectErr: true,
		},
		{
			name:      "empty output",
			output:    "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDocPRURL(tt.output)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
