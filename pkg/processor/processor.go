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
)

type TaskProcessor struct {
	githubClient       *github.Client
	devWorkspace       *devworkspace.DevWorkspace
	commandTemplates   map[commands.SubCommandType]string
	taskTimeout        time.Duration
	outputDir          string
	deleteDevWorkspace bool
}

const (
	devWorkspaceNamePrefix = "che-ai"
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
		outputDir:          cfg.OutputDir,
		deleteDevWorkspace: cfg.DeleteDevWorkspace,
	}, nil
}

func (p *TaskProcessor) Trigger(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[INFO] Running %s for %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.IssueNumber)

	if !commands.IsCommandAvailableForRepo(trigger.SubCommandType, trigger.Owner+"/"+trigger.Repo) {
		p.processHandleUnavailable(ctx, trigger)
		return
	}

	switch trigger.SubCommandType {
	case commands.SubCommandHelp:
		p.processHelp(ctx, trigger)
	case commands.SubCommandImplement:
		p.processImplement(ctx, trigger)
	default:
		p.processDefault(ctx, trigger)
	}
}

func (p *TaskProcessor) processImplement(
	ctx context.Context,
	trigger *github.Trigger,
) {
	devWorkspaceName := fmt.Sprintf(
		"%s-%s-%s-%d",
		devWorkspaceNamePrefix,
		trigger.SubCommandType,
		trigger.Repo,
		trigger.IssueNumber,
	)

	defer func() {
		if p.deleteDevWorkspace {
			err := p.devWorkspace.Delete(ctx, devWorkspaceName)
			if err != nil {
				log.Printf("[ERROR] Failed to delete the DevWorkspace %s: %v", devWorkspaceName, err)
			}
		}
	}()

	err := p.devWorkspace.StartWithRepository(ctx, devWorkspaceName, "https://github.com/akurinnoy/supervisor-terminal", "main")
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	command := fmt.Sprintf("cd /projects/supervisor-terminal; /start.sh --url %s --auto-approve --effort-override high", trigger.IssueURL)
	err = p.devWorkspace.Exec(ctx, devWorkspaceName, command)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	err = p.devWorkspace.WaitExecFinished(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	p.OnSuccess(ctx, devWorkspaceName, trigger)
}

func (p *TaskProcessor) processDefault(
	ctx context.Context,
	trigger *github.Trigger,
) {
	devWorkspaceName := fmt.Sprintf(
		"%s-%s-%s-%d",
		devWorkspaceNamePrefix,
		trigger.SubCommandType,
		trigger.Repo,
		trigger.IssueNumber,
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
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	err = p.devWorkspace.Start(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	err = p.devWorkspace.CopyClaudeConfigInDevWorkspace(ctx, devWorkspaceName)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	err = p.devWorkspace.RunClaudeTask(ctx, devWorkspaceName, task)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	err = p.devWorkspace.WaitTaskFinished(ctx, devWorkspaceName, p.taskTimeout)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger)
		return
	}

	p.OnSuccess(ctx, devWorkspaceName, trigger)
}

func (p *TaskProcessor) OnSuccess(
	ctx context.Context,
	devWorkspaceName string,
	trigger *github.Trigger,
) {
	outputFile := filepath.Join(p.outputDir, fmt.Sprintf("workspace-output-%d.txt", time.Now().UnixNano()))
	if output, err := p.devWorkspace.ReadWorkspaceOutput(ctx, devWorkspaceName); err != nil {
		log.Printf("[ERROR] Failed to read the output in the DevWorkspace %s: %v", devWorkspaceName, err)
	} else {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			log.Printf("[ERROR] Failed to write DevWorkspace output to %s: %v", outputFile, err)
		}
	}

	var issueOrPR string
	if trigger.IsIssue {
		issueOrPR = "issues"
	} else {
		issueOrPR = "pull"
	}

	log.Printf(
		"[INFO] %s completed for comment https://github.com/%s/%s/%s/%d#issuecomment-%d, output %s",
		trigger.SubCommandType,
		trigger.Owner,
		trigger.Repo,
		issueOrPR,
		trigger.IssueNumber,
		trigger.CommentID,
		outputFile,
	)

	body := fmt.Sprintf("%s\n\nTask completed.", trigger.CommentBody)
	if err := p.githubClient.UpdateComment(
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
			trigger.IssueNumber,
			err,
		)
	}
}

func (p *TaskProcessor) onError(
	ctx context.Context,
	devWorkspaceName string,
	err error,
	trigger *github.Trigger,
) {
	outputFile := filepath.Join(p.outputDir, fmt.Sprintf("workspace-output-%d.txt", time.Now().UnixNano()))
	if output, err := p.devWorkspace.ReadWorkspaceOutput(ctx, devWorkspaceName); err != nil {
		log.Printf("[ERROR] Failed to read the output in the DevWorkspace %s: %v", devWorkspaceName, err)
	} else {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			log.Printf("[ERROR] Failed to write DevWorkspace output to %s: %v", outputFile, err)
		}
	}

	var issueOrPR string
	if trigger.IsIssue {
		issueOrPR = "issues"
	} else {
		issueOrPR = "pull"
	}

	log.Printf(
		"[INFO] %s failed for comment https://github.com/%s/%s/%s/%d#issuecomment-%d, output %s, error %v",
		trigger.SubCommandType,
		trigger.Owner,
		trigger.Repo,
		issueOrPR,
		trigger.IssueNumber,
		trigger.CommentID,
		outputFile,
		err,
	)

	body := fmt.Sprintf("%s\n\nTask failed.", trigger.CommentBody)
	if err := p.githubClient.UpdateComment(
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
			trigger.IssueNumber,
			err,
		)
	}
}

func (p *TaskProcessor) processHelp(ctx context.Context, trigger *github.Trigger) {
	err := p.githubClient.PostWelcomeComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.IsIssue,
		trigger.IssueNumber,
	)

	if err != nil {
		log.Printf(
			"[ERROR] Failed to post on %s/%s#%d: %v",
			trigger.Owner,
			trigger.Repo,
			trigger.IssueNumber,
			err,
		)
	}
}

func (p *TaskProcessor) processUnknown(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.IssueNumber)
}

func (p *TaskProcessor) processHandleUnavailable(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] command %q is unavailable on %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.IssueNumber)
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
		"PullRequestURL": trigger.IssueURL,
		"IssueURL":       trigger.IssueURL,
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
		name := strings.TrimSuffix(file.Name(), ".tmpl")
		commandsTemplates[commands.SubCommandType(name)] = content
	}

	if len(commandsTemplates) == 0 {
		return nil, fmt.Errorf("no templates found in %s", dir)
	}

	return commandsTemplates, nil
}
