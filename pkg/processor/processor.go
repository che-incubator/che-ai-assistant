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
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/devworkspace"
	"github.com/tolusha/che-doc-generator/pkg/github"
	"github.com/tolusha/che-doc-generator/pkg/handlers"
)

type TaskProcessor struct {
	githubClient     *github.Client
	devWorkspace     *devworkspace.DevWorkspace
	commandTemplates map[commands.SubCommandType]string
	taskTimeout      time.Duration
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

func NewTaskProcessor(cfg *config.Config) (*TaskProcessor, error) {
	templates, err := loadTemplates(cfg.TemplatesDir)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to load templates"), err)
	}

	return &TaskProcessor{
		githubClient:     github.NewGitHubClient(cfg),
		devWorkspace:     devworkspace.NewDevWorkspace(cfg),
		commandTemplates: templates,
		taskTimeout:      cfg.TaskTimeout,
	}, nil
}

func (p *TaskProcessor) Trigger(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[INFO] Running %s for %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)

	switch trigger.SubCommand {
	case commands.SubCommandHelp:
		p.HandleHelp(ctx, trigger)
	default:
		handler := commandHandlers[trigger.SubCommand]
		if handler == nil {
			p.HandleUnknown(ctx, trigger)
			return
		}

		p.handle(ctx, trigger, handler)
	}
}

func (p *TaskProcessor) handle(
	ctx context.Context,
	trigger *github.Trigger,
	handler handlers.Handler,
) {
	devWorkspaceName := fmt.Sprintf(
		"%s-%s-%s-%d",
		devWorkspaceNamePrefix,
		trigger.SubCommand,
		trigger.Repo,
		trigger.PRNumber,
	)

	task, err := p.buildPrompt(trigger)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger)
		return
	}

	err = p.startDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger)
		return
	}

	err = p.copyClaudeConfigInDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger)
		return
	}

	err = p.runTaskInDevWorkspace(ctx, task, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger)
		return
	}

	err = p.waiteTaskFinishedInDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger)
		return
	}

	output, err := p.readTaskOutputInDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger)
		return
	}

	p.OnSuccess(ctx, output, trigger, handler)

	err = p.deleteDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		log.Printf("[ERROR] Failed to delete the DevWorkspace %s: %v", devWorkspaceName, err)
	}
}

func (p *TaskProcessor) OnSuccess(
	ctx context.Context,
	output string,
	trigger *github.Trigger,
	handler handlers.Handler,
) {
	log.Printf(
		"[INFO] %s completed for %s/%s#%d",
		trigger.SubCommand,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
	)

	handler.OnSuccess(
		ctx,
		output,
		trigger,
		p.githubClient,
	)
}

func (p *TaskProcessor) onError(
	ctx context.Context,
	err error,
	devWorkspaceName string,
	trigger *github.Trigger,
) {
	log.Printf(
		"[ERROR] %s failed for %s/%s#%d: %v",
		trigger.SubCommand,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		err,
	)

	body := fmt.Sprintf("%s\n\nCommand failed.", trigger.CommentBody)
	if err := p.githubClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		body,
	); err != nil {
		log.Printf(
			"[ERROR] Failed to post on %s/%s#%d: %v",
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
			err,
		)
	}

	err = p.deleteDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		log.Printf("[ERROR] Failed to delete the DevWorkspace %s: %v", devWorkspaceName, err)
	}
}

func (p *TaskProcessor) HandleHelp(ctx context.Context, trigger *github.Trigger) {
	err := p.githubClient.PostPullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		commands.BuildWelcomeMessage(),
	)

	if err != nil {
		log.Printf(
			"[ERROR] Failed to post on %s/%s#%d: %v",
			trigger.Owner,
			trigger.Repo,
			trigger.PRNumber,
			err,
		)
	}
}

func (p *TaskProcessor) HandleUnknown(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)
}

func (p *TaskProcessor) buildPrompt(trigger *github.Trigger) (string, error) {
	commandTemplateContent, ok := p.commandTemplates[trigger.SubCommand]
	if !ok {
		return "", fmt.Errorf("no template found for subcommand %q", trigger.SubCommand)
	}

	commandTemplate, err := template.New("prompt").Parse(commandTemplateContent)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %w", err)
	}

	var prompt strings.Builder
	data := map[string]string{
		"PullRequestURL": trigger.PullRequestURL,
	}

	if err := commandTemplate.Execute(&prompt, data); err != nil {
		return "", fmt.Errorf("prompt template execution failed: %w", err)
	}

	return prompt.String(), nil
}

func loadTemplates(dir string) (map[commands.SubCommandType]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading templates directory: %w", err)
	}

	commandsTemplates := make(map[commands.SubCommandType]string)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".tmpl") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", file.Name(), err)
		}

		content := strings.TrimSpace(string(data))
		if !strings.Contains(content, "{{.PullRequestURL}}") {
			return nil, fmt.Errorf("template %s must contain {{.PullRequestURL}} placeholder", file.Name())
		}

		name := strings.TrimSuffix(file.Name(), ".tmpl")
		commandsTemplates[commands.SubCommandType(name)] = content
	}

	if len(commandsTemplates) == 0 {
		return nil, fmt.Errorf("no templates found in %s", dir)
	}

	return commandsTemplates, nil
}
