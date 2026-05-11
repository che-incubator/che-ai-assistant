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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/github"
)

func writeTemplateFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestNew_LoadsTemplatesFromDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "generate-che-doc.tmpl", "Generate docs for {{.PullRequestURL}}")

	cfg := &config.Config{TemplatesDir: dir}

	h, err := New(nil, cfg)

	require.NoError(t, err)
	assert.Contains(t, h.templates, commands.SubCommandGenerateCheDoc)
	assert.Equal(t, "Generate docs for {{.PullRequestURL}}", h.templates[commands.SubCommandGenerateCheDoc])
}

func TestNew_LoadsMultipleTemplates(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "generate-che-doc.tmpl", "Generate docs for {{.PullRequestURL}}")
	writeTemplateFile(t, dir, "review.tmpl", "Review PR {{.PullRequestURL}}")

	cfg := &config.Config{TemplatesDir: dir}

	h, err := New(nil, cfg)

	require.NoError(t, err)
	assert.Len(t, h.templates, 2)
	assert.Contains(t, h.templates, commands.SubCommandGenerateCheDoc)
	assert.Contains(t, h.templates, commands.SubCommandType("review"))
}

func TestNew_EmptyTemplate(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "generate-che-doc.tmpl", "   ")

	cfg := &config.Config{TemplatesDir: dir}

	h, err := New(nil, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
	assert.Nil(t, h)
}

func TestNew_MissingPRURLPlaceholder(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "generate-che-doc.tmpl", "Generate docs for this PR")

	cfg := &config.Config{TemplatesDir: dir}

	h, err := New(nil, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain {{.PullRequestURL}}")
	assert.Nil(t, h)
}

func TestNew_MissingDirectory(t *testing.T) {
	cfg := &config.Config{TemplatesDir: "/nonexistent/templates"}

	h, err := New(nil, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading templates directory")
	assert.Nil(t, h)
}

func TestNew_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{TemplatesDir: dir}

	h, err := New(nil, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no templates found")
	assert.Nil(t, h)
}

func TestNew_IgnoresNonTmplFiles(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "generate-che-doc.tmpl", "Generate docs for {{.PullRequestURL}}")
	writeTemplateFile(t, dir, "README.md", "This is not a template")

	cfg := &config.Config{TemplatesDir: dir}

	h, err := New(nil, cfg)

	require.NoError(t, err)
	assert.Len(t, h.templates, 1)
}

func TestBuildPrompt(t *testing.T) {
	h := &Processor{templates: map[commands.SubCommandType]string{
		commands.SubCommandGenerateCheDoc: "Generate docs for {{.PullRequestURL}} in {{.DevWorkspaceName}} please",
	}}

	trigger := &github.Trigger{
		Owner:          "org",
		Repo:           "repo",
		PRNumber:       42,
		PullRequestURL: "https://github.com/org/repo/pull/42",
		SubCommand:     commands.SubCommandGenerateCheDoc,
	}

	result, err := h.buildPrompt(trigger)

	require.NoError(t, err)
	assert.Equal(t, "Generate docs for https://github.com/org/repo/pull/42 in generate-che-doc-repo-pr-42 please", result)
}

func TestBuildPrompt_MissingTemplate(t *testing.T) {
	h := &Processor{templates: map[commands.SubCommandType]string{}}

	trigger := &github.Trigger{
		SubCommand: commands.SubCommandGenerateCheDoc,
	}

	_, err := h.buildPrompt(trigger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template found")
}
