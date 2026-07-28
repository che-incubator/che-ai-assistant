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
	"che-incubator/che-ai-assistant/pkg/commands"
	"che-incubator/che-ai-assistant/pkg/common"
	"che-incubator/che-ai-assistant/pkg/config"
	"che-incubator/che-ai-assistant/pkg/devworkspace"
	"che-incubator/che-ai-assistant/pkg/github"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

type TaskProcessor struct {
	githubClient           *github.Client
	devWorkspace           *devworkspace.DevWorkspace
	prompts                map[commands.SubCommandType]string
	taskTimeout            time.Duration
	skillsRepositoryUrl    string
	skillsRepositoryBranch string
}

const (
	devWorkspaceNamePrefix = "che-ai"
)

var (
	repositoryNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	emptyTaskOutputReader = func(ctx context.Context, s string) (string, error) { return "", nil }
)

func NewTaskProcessor(cfg *config.Config) (*TaskProcessor, error) {
	prompts, err := loadPrompts(cfg.PromptsDir)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to load prompts"), err)
	}

	return &TaskProcessor{
		githubClient:           github.NewGitHubClient(cfg),
		devWorkspace:           devworkspace.NewDevWorkspace(cfg),
		prompts:                prompts,
		taskTimeout:            cfg.TaskTimeout,
		skillsRepositoryUrl:    cfg.SkillsRepositoryUrl,
		skillsRepositoryBranch: cfg.SkillsRepositoryBranch,
	}, nil
}

func (p *TaskProcessor) Trigger(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[INFO] Running %s for %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.IssueNumber)

	if !commands.IsKnownCommand(trigger.SubCommandType) {
		p.processUnknown(ctx, trigger)
		return
	}

	if !commands.IsCommandAvailableForRepo(trigger.SubCommandType, trigger.Owner+"/"+trigger.Repo) {
		p.processUnknown(ctx, trigger)
		return
	}

	switch trigger.SubCommandType {
	case commands.SubCommandHelp:
		p.processHelp(ctx, trigger)
	default:
		p.processDefault(ctx, trigger)
	}
}

func (p *TaskProcessor) processDefault(
	ctx context.Context,
	trigger *github.Trigger,
) {
	if trigger.SubCommandType == commands.SubCommandClaude && trigger.Args == "" {
		body := fmt.Sprintf("%s\n\nError: the `claude` command requires an instruction. Usage: `%s claude <instruction>`", trigger.CommentBody, commands.Command)
		if _, err := p.githubClient.CreateComment(ctx, trigger.Owner, trigger.Repo, trigger.IssueNumber, body); err != nil {
			log.Printf("[ERROR] Failed to post on %s/%s#%d: %v", trigger.Owner, trigger.Repo, trigger.IssueNumber, err)
		}

		return
	}

	devWorkspaceName := fmt.Sprintf(
		"%s-%s-%s-%d",
		devWorkspaceNamePrefix,
		trigger.SubCommandType,
		trigger.Repo,
		trigger.IssueNumber,
	)

	defer func() {
		// Use a new context (not to use canceled context occasionally)
		err := p.devWorkspace.Delete(context.Background(), devWorkspaceName)
		if err != nil {
			log.Printf("[ERROR] Failed to delete the DevWorkspace %s: %v", devWorkspaceName, err)
		}
	}()

	task, err := p.buildPrompt(trigger)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger, emptyTaskOutputReader)
		return
	}

	_, skillsRepositoryName := common.ParseRepoSlug(p.skillsRepositoryUrl)
	if !repositoryNamePattern.MatchString(skillsRepositoryName) {
		p.onError(ctx, devWorkspaceName, fmt.Errorf("repository %s name doesn't match pattern", skillsRepositoryName), trigger, emptyTaskOutputReader)
		return
	}

	copyClaudeConfigCommand := fmt.Sprintf("cp -r /home/user/projects/%s/.claude /home/user/ && rm -rf /home/user/projects/%s", skillsRepositoryName, skillsRepositoryName)

	err = p.devWorkspace.StartFromRepository(
		ctx,
		devWorkspaceName,
		p.skillsRepositoryUrl,
		p.skillsRepositoryBranch,
		copyClaudeConfigCommand,
	)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger, emptyTaskOutputReader)
		return
	}

	err = p.devWorkspace.EnsureRunning(ctx, devWorkspaceName, 5*time.Minute)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger, emptyTaskOutputReader)
		return
	}

	err = p.devWorkspace.RunClaudeTask(ctx, devWorkspaceName, task)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger, p.devWorkspace.ReadWorkspaceAgentOutput)
		return
	}

	err = p.devWorkspace.WaitTaskFinished(ctx, devWorkspaceName, p.taskTimeout)
	if err != nil {
		p.onError(ctx, devWorkspaceName, err, trigger, p.devWorkspace.ReadWorkspaceAgentOutput)
		return
	}

	p.onSuccess(ctx, devWorkspaceName, trigger, p.devWorkspace.ReadWorkspaceAgentOutput)
}

func (p *TaskProcessor) onSuccess(
	ctx context.Context,
	devWorkspaceName string,
	trigger *github.Trigger,
	readTaskOutput func(context.Context, string) (string, error),
) {
	outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("workspace-output-%d.txt", time.Now().UnixNano()))
	if output, err := readTaskOutput(ctx, devWorkspaceName); err != nil {
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
	readTaskOutput func(context.Context, string) (string, error),
) {
	outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("workspace-output-%d.txt", time.Now().UnixNano()))
	if output, err := readTaskOutput(ctx, devWorkspaceName); err != nil {
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

func (p *TaskProcessor) processUnknown(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommandType, trigger.Owner, trigger.Repo, trigger.IssueNumber)
	if err := p.githubClient.PostWelcomeComment(ctx, trigger.Owner, trigger.Repo, trigger.IsIssue, trigger.IssueNumber); err != nil {
		log.Printf("[ERROR] failed to post welcome comment: %v, issue %s", err, trigger.IssueURL)
	}
}

func (p *TaskProcessor) buildPrompt(trigger *github.Trigger) (string, error) {
	prompt, ok := p.prompts[trigger.SubCommandType]
	if !ok {
		return "", fmt.Errorf("no prompt found for subcommand %q", trigger.SubCommandType)
	}

	commandTemplate, err := template.New("prompt").Parse(prompt)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %w", err)
	}

	var builder strings.Builder
	data := map[string]string{
		"PullRequestURL": trigger.IssueURL,
		"IssueURL":       trigger.IssueURL,
		"Args":           trigger.Args,
	}

	if err := commandTemplate.Execute(&builder, data); err != nil {
		return "", fmt.Errorf("prompt template execution failed: %w", err)
	}

	return builder.String(), nil
}

func loadPrompts(dir string) (map[commands.SubCommandType]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading prompts directory: %w", err)
	}

	prompts := make(map[commands.SubCommandType]string)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".tmpl") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading prompts %s: %w", file.Name(), err)
		}

		content := strings.TrimSpace(string(data))
		name := strings.TrimSuffix(file.Name(), ".tmpl")
		prompts[commands.SubCommandType(name)] = content
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf("no prompts found in %s", dir)
	}

	return prompts, nil
}
