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

	"github.com/tolusha/che-doc-generator/pkg/claude"
	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/devworkspace"
	"github.com/tolusha/che-doc-generator/pkg/github"
	"github.com/tolusha/che-doc-generator/pkg/handlers"
)

type TaskProcessor struct {
	githubClient       *github.Client
	devWorkspace       *devworkspace.DevWorkspace
	commandTemplates   map[commands.SubCommandType]string
	taskTimeout        time.Duration
	claudeOutputDir    string
	deleteDevWorkspace bool
}

const (
	devWorkspaceNamePrefix = "che-ai"
)

var (
	commandHandlers = map[commands.SubCommandType]handlers.Handler{
		commands.SubCommandGenerateCheDoc:       handlers.NewGenerateCheDocHandler(),
		commands.SubCommandPullRequestReview:    handlers.NewOkPRReviewHandler(),
		commands.SubCommandPullRequestReadiness: handlers.NewOkPRReadinessHandler(),
		commands.SubCommandCheckPRTestFailures:  handlers.NewCheckPRTestFailuresHandler(),
		commands.SubCommandUpdateCheE2ETest:     handlers.NewUpdateCheE2ETestHandler(),
	}
)

func NewTaskProcessor(cfg *config.Config) (*TaskProcessor, error) {
	templates, err := loadTemplates(cfg.TemplatesDir)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to load templates"), err)
	}

	return &TaskProcessor{
		githubClient:       github.NewGitHubClient(cfg),
		devWorkspace:       devworkspace.NewDevWorkspace(cfg),
		commandTemplates:   templates,
		taskTimeout:        cfg.TaskTimeout,
		claudeOutputDir:    cfg.ClaudeOutputDir,
		deleteDevWorkspace: cfg.DeleteDevWorkspace,
	}, nil
}

func (p *TaskProcessor) Trigger(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[INFO] Running %s for %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.PRNumber)

	switch trigger.SubCommandType {
	case commands.SubCommandHelp:
		p.HandleHelp(ctx, trigger)
	default:
		if !commands.IsCommandAvailableForRepo(trigger.SubCommandType, trigger.Owner+"/"+trigger.Repo) {
			p.HandleUnavailable(ctx, trigger)
			return
		}

		handler := commandHandlers[trigger.SubCommandType]
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
		trigger.SubCommandType,
		trigger.Repo,
		trigger.PRNumber,
	)

	defer func() {
		if p.deleteDevWorkspace {
			err := p.devWorkspace.Delete(ctx, devWorkspaceName)
			if err != nil {
				log.Printf("[ERROR] Failed to delete the DevWorkspace %s: %v", devWorkspaceName, err)
			}
		}
	}()

	task, err := p.buildPrompt(trigger)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger, handler)
		return
	}

	err = p.devWorkspace.Start(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger, handler)
		return
	}

	err = p.devWorkspace.CopyClaudeConfigInDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger, handler)
		return
	}

	err = p.devWorkspace.RunClaudeTask(ctx, devWorkspaceName, task)
	if err != nil {
		p.onError(ctx, err, devWorkspaceName, trigger, handler)
		return
	}

	waitTaskFinishedErr := p.waitTaskFinishedInDevWorkspace(ctx, devWorkspaceName)

	output, readClaudeTaskOutputErr := p.devWorkspace.ReadClaudeTaskOutput(ctx, devWorkspaceName)

	if waitTaskFinishedErr != nil {
		p.onError(ctx, waitTaskFinishedErr, devWorkspaceName, trigger, handler)
		return
	} else if readClaudeTaskOutputErr != nil {
		p.onError(ctx, readClaudeTaskOutputErr, devWorkspaceName, trigger, handler)
		return
	}

	p.OnSuccess(ctx, output, trigger, handler)
}

func (p *TaskProcessor) OnSuccess(
	ctx context.Context,
	output string,
	trigger *github.Trigger,
	handler handlers.Handler,
) {
	outputFile := filepath.Join(
		p.claudeOutputDir,
		fmt.Sprintf("claude-%d.txt", time.Now().UnixNano()),
	)

	if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
		log.Printf("[ERROR] Failed to write Claude task output to %s: %v", outputFile, err)
	}

	log.Printf(
		"[INFO] %s completed for %s/%s#%d, see https://github.com/%s/%s/pull/%d#issuecomment-%d, output %s",
		trigger.SubCommandType,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		trigger.CommentID,
		outputFile,
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
	handler handlers.Handler,
) {
	log.Printf(
		"[ERROR] %s failed for %s/%s#%d: %v",
		trigger.SubCommandType,
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

	handler.OnError(
		ctx,
		trigger,
		p.githubClient,
	)
}

func (p *TaskProcessor) HandleHelp(ctx context.Context, trigger *github.Trigger) {
	err := p.githubClient.PostPullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		commands.BuildWelcomeMessage(trigger.Owner+"/"+trigger.Repo),
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
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.PRNumber)
}

func (p *TaskProcessor) HandleUnavailable(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] command %q is unavailable on %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.PRNumber)
}

func (p *TaskProcessor) buildPrompt(trigger *github.Trigger) (string, error) {
	commandTemplateContent, ok := p.commandTemplates[trigger.SubCommandType]
	if !ok {
		return "", fmt.Errorf("no template found for subcommand %q", trigger.SubCommandType)
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

func (p *TaskProcessor) waitTaskFinishedInDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	ctx, cancel := context.WithTimeout(ctx, p.taskTimeout)
	defer cancel()

	start := time.Now()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for task to finish in the DevWorkspace %s", devWorkspaceName)
		case <-ticker.C:
			log.Printf("[INFO] Waiting for task to finish in the DevWorkspace %s (elapsed: %s)", devWorkspaceName, time.Since(start).Round(time.Second))
			status, err := p.devWorkspace.ReadClaudeTaskStatus(ctx, devWorkspaceName)
			if err != nil {
				return errors.Join(fmt.Errorf("failed to read task status in the DevWorkspace %s", devWorkspaceName), err)
			}

			switch status {
			case claude.StatusRunning:
				continue
			case claude.StatusFinished:
				log.Printf("[INFO] Task finished in the DevWorkspace %s, lasted %s", devWorkspaceName, time.Since(start).Round(time.Second))
				return nil
			default:
				return fmt.Errorf("unexpected task status %s in the DevWorkspace %s", status, devWorkspaceName)
			}
		}
	}
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
