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
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/github"
	"github.com/tolusha/che-doc-generator/pkg/processor/handlers"
)

type Handler struct {
	ghClient     *github.Client
	timeout      time.Duration
	pollInterval time.Duration
	templates    map[commands.SubCommandType]string
}

func New(
	ghClient *github.Client,
	cfg *config.Config,
) (*Handler, error) {
	templates, err := loadTemplates(cfg.TemplatesDir)
	if err != nil {
		return nil, err
	}

	return &Handler{
		ghClient:     ghClient,
		timeout:      cfg.TaskTimeout,
		pollInterval: cfg.PollInterval,
		templates:    templates,
	}, nil
}

func (h *Handler) Trigger(ctx context.Context, trigger *github.Trigger) {
	deps := &handlers.HandlerDependency{
		GHClient:     h.ghClient,
		Timeout:      h.timeout,
		PollInterval: h.pollInterval,
		BuildPrompt:  h.buildPrompt,
	}

	log.Printf("[INFO] running %s for %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)

	switch trigger.SubCommand {
	case commands.SubCommandHelp:
		handlers.HandleHelp(ctx, trigger, deps)
	case commands.SubCommandGenerateCheDoc:
		handlers.HandleGenerateCheDoc(ctx, trigger, deps)
	default:
		handlers.HandleUnknown(ctx, trigger)
	}
}

func (h *Handler) buildPrompt(subCommand commands.SubCommandType, prUrl string) (string, error) {
	tmplContent, ok := h.templates[subCommand]
	if !ok {
		return "", fmt.Errorf("no template found for subcommand %q", subCommand)
	}

	tmpl, err := template.New("prompt").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %w", err)
	}

	var buf strings.Builder
	data := map[string]string{"PRURL": prUrl}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompt template execution failed: %w", err)
	}

	return buf.String(), nil
}

func loadTemplates(dir string) (map[commands.SubCommandType]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading templates directory: %w", err)
	}

	templates := make(map[commands.SubCommandType]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", entry.Name(), err)
		}

		content := strings.TrimSpace(string(data))
		if content == "" {
			return nil, fmt.Errorf("template %s is empty", entry.Name())
		}

		if !strings.Contains(content, "{{.PRURL}}") {
			return nil, fmt.Errorf("template %s must contain {{.PRURL}} placeholder", entry.Name())
		}

		name := strings.TrimSuffix(entry.Name(), ".tmpl")
		templates[commands.SubCommandType(name)] = content
	}

	if len(templates) == 0 {
		return nil, fmt.Errorf("no templates found in %s", dir)
	}

	return templates, nil
}
