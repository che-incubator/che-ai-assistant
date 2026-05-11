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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/github"
	"github.com/tolusha/che-doc-generator/pkg/processor/handlers"
)

type Processor struct {
	ghClient     *github.Client
	timeout      time.Duration
	pollInterval time.Duration
	templates    map[commands.SubCommandType]string
}

type ClaudeOutput struct {
	Result string `json:"result"`
}

const (
	devWorkspaceNamePrefix = "che-ai"
)

var (
	commandHandlers = map[commands.SubCommandType]handlers.Handler{
		commands.SubCommandGenerateCheDoc:    handlers.NewGenerateCheDocHandler(),
		commands.SubCommandPullRequestReview: handlers.NewOkPRReviewHandler(),
	}
)

func New(
	ghClient *github.Client,
	cfg *config.Config,
) (*Processor, error) {
	templates, err := loadTemplates(cfg.TemplatesDir)
	if err != nil {
		return nil, err
	}

	return &Processor{
		ghClient:     ghClient,
		timeout:      cfg.TaskTimeout,
		pollInterval: cfg.PollInterval,
		templates:    templates,
	}, nil
}

func (p *Processor) Trigger(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[INFO] running %s for %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)

	switch trigger.SubCommand {
	case commands.SubCommandHelp:
		p.HandleHelp(ctx, trigger)
	default:
		handler := commandHandlers[trigger.SubCommand]
		if handler == nil {
			p.HandleUnknown(ctx, trigger)
			return
		}

		p.run(ctx, trigger, handler)
	}
}

func (p *Processor) run(ctx context.Context, trigger *github.Trigger, handler handlers.Handler) {
	prompt, err := p.buildPrompt(trigger)
	if err != nil {
		log.Printf("[ERROR] failed to build prompt for %s: %s", trigger.SubCommand, err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	log.Printf("[INFO] Claude prompt >>>>>>>>")
	log.Printf("%s", prompt)
	log.Printf("[INFO] Claude prompt <<<<<<<<")

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")
	data, err := cmd.CombinedOutput()

	claudeOutputFile := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(
			"claude-output-%s-%s-%d-%d.json",
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
			time.Now().Unix(),
		),
	)
	if writeErr := os.WriteFile(claudeOutputFile, data, 0644); writeErr != nil {
		log.Printf(
			"[ERROR] failed to write claude output to %s: %v",
			claudeOutputFile,
			writeErr,
		)
	} else {
		log.Printf(
			"[INFO] Claude output written to %s for %s/%s#%d",
			claudeOutputFile,
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
		)
	}

	if err != nil {
		log.Printf(
			"[ERROR] %s failed for %s/%s#%d: %v",
			trigger.SubCommand,
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
			err,
		)

		body := fmt.Sprintf("%s\n\nCommand fialed.", trigger.CommentBody)
		if err := p.ghClient.UpdatePullRequestComment(
			ctx,
			trigger.Owner,
			trigger.Repo,
			trigger.CommentID,
			body,
		); err != nil {
			log.Printf(
				"[ERROR] Failed to post on %s/%s#%d",
				trigger.Owner,
				trigger.Repo,
				trigger.PRNumber,
			)
		}

		handler.OnError(
			ctx,
			trigger,
			p.ghClient,
		)
	} else {
		var claudeOutput ClaudeOutput
		if err := json.Unmarshal(data, &claudeOutput); err != nil {
			log.Printf("[ERROR] failed to unmarshal claude output: %s", err)
			return
		}

		log.Printf(
			"[INFO] %s completed for %s/%s#%d",
			trigger.SubCommand,
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
		)

		handler.OnSuccess(
			ctx,
			claudeOutput.Result,
			trigger,
			p.ghClient,
		)
	}
}

func (p *Processor) HandleHelp(ctx context.Context, trigger *github.Trigger) {
	if err := p.ghClient.PostPullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		commands.BuildWelcomeMessage(p.pollInterval),
	); err != nil {
		log.Printf("[ERROR] error posting help comment: %v", err)
	}
}

func (p *Processor) HandleUnknown(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)
}

func (p *Processor) buildPrompt(trigger *github.Trigger) (string, error) {
	tmplContent, ok := p.templates[trigger.SubCommand]
	if !ok {
		return "", fmt.Errorf("no template found for subcommand %q", trigger.SubCommand)
	}

	tmpl, err := template.New("prompt").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %w", err)
	}

	devWorkspaceName := fmt.Sprintf(
		"%s-%s-%s-%d",
		devWorkspaceNamePrefix,
		trigger.SubCommand,
		trigger.Repo,
		trigger.PRNumber,
	)

	var buf strings.Builder
	data := map[string]string{
		"PullRequestURL":   trigger.PullRequestURL,
		"DevWorkspaceName": devWorkspaceName,
	}

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

		if !strings.Contains(content, "{{.PullRequestURL}}") {
			return nil, fmt.Errorf("template %s must contain {{.PullRequestURL}} placeholder", entry.Name())
		}

		name := strings.TrimSuffix(entry.Name(), ".tmpl")
		templates[commands.SubCommandType(name)] = content
	}

	if len(templates) == 0 {
		return nil, fmt.Errorf("no templates found in %s", dir)
	}

	return templates, nil
}
