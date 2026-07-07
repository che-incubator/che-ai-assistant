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

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
	"time"

	gh "github.com/google/go-github/v68/github"
	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/github"
	"github.com/tolusha/che-doc-generator/pkg/processor"
	"github.com/tolusha/che-doc-generator/pkg/scanner"
)

var (
	githubRepository = regexp.MustCompile(`^(?:https?://[^/]+/)?([^/]+)/([^/]+?)(?:\.git)?$`)
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("[ERROR] Failed to read configuration: %v", err)
	}

	setupLogging(cfg.LogFile)
	defer func() {
		_ = log.Writer().(*os.File).Close()
	}()

	ghClient := github.NewGitHubClient(cfg)

	taskProcessor, err := processor.NewTaskProcessor(cfg)
	if err != nil {
		log.Fatalf("[ERROR] processor.NewTaskProcessor: %v", err)
	}

	log.Printf("[INFO] starting che-ai-assistant: watching %v, poll every %v", cfg.GitHubWatchRepos, cfg.TasksPollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	var wg sync.WaitGroup

	poll := pollFunc(ctx, &wg, cfg, ghClient, taskProcessor)

	ticker := time.NewTicker(cfg.TasksPollInterval)
	defer ticker.Stop()

	poll()

	for {
		select {
		case <-ticker.C:
			poll()

		case <-sigCh:
			cancel()
			wg.Wait()

			return
		}
	}
}

func setupLogging(logFilePath string) {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("[ERROR] error opening file: %v", err)
	}

	log.SetOutput(logFile)
}

func pollFunc(
	ctx context.Context,
	wg *sync.WaitGroup,
	cfg *config.Config,
	ghClient *github.Client,
	processor *processor.TaskProcessor,
) func() {
	sem := make(chan struct{}, cfg.MaxConcurrentTasks)

	dispatchTrigger := func(trigger *github.Trigger) {
		wg.Add(1)
		go func(trigger *github.Trigger) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			processor.Trigger(ctx, trigger)
		}(trigger)
	}

	return func() {
		for _, repositoryUrl := range cfg.GitHubWatchRepos {
			owner, repo := parseRepoSlug(repositoryUrl)
			if owner == "" || repo == "" {
				log.Printf("[ERROR] invalid repo format: %s (expected owner/repo or https://github.com/owner/repo)", repositoryUrl)
				continue
			}

			pollPullRequests(ctx, owner, repo, cfg, ghClient, dispatchTrigger)
			pollIssues(ctx, owner, repo, ghClient, dispatchTrigger)
		}
	}
}

func pollPullRequests(
	ctx context.Context,
	owner, repo string,
	cfg *config.Config,
	ghClient *github.Client,
	dispatchTrigger func(*github.Trigger),
) {
	pullRequests, err := ghClient.GetPullRequests(ctx, owner, repo)
	if err != nil {
		log.Printf("[ERROR] failed to fetch pull requests: %v, owner %s, repo %s", err, owner, repo)
		return
	}

	for _, pullRequest := range pullRequests {
		comments, err := ghClient.GetComments(ctx, owner, repo, *pullRequest.Number)
		if err != nil {
			log.Printf("[ERROR] failed to fetch comments: %v, owner %s, repo %s, pr %d", err, owner, repo, pullRequest.GetNumber())
			continue
		}

		// post welcome message
		if ghClient.IsPullRequestAuthorEligible(pullRequest) && !ghClient.HasWelcomeComment(comments) {
			log.Printf("[INFO] posting welcome comment on %s/%s#%d", owner, repo, pullRequest.GetNumber())

			err := ghClient.PostWelcomeComment(ctx, owner, repo, false, pullRequest.GetNumber())
			if err != nil {
				log.Printf("[ERROR] failed to post welcome comment: %v, owner %s, repo %s, pr %d", err, owner, repo, pullRequest.GetNumber())
			}
		}

		// post warning message
		if !ghClient.HasWarningComment(comments) {
			files, err := ghClient.GetPullRequestFiles(ctx, owner, repo, pullRequest.GetNumber())
			if err != nil {
				log.Printf("[ERROR] failed to fetch pull request files: %v, owner %s, repo %s, pr %d", err, owner, repo, pullRequest.GetNumber())
			} else {
				matched := scanner.CheckFiles(files, cfg.WarnDirsCommits)
				if len(matched) > 0 {
					log.Printf("[INFO] posting warning comment on %s/%s#%d for files: %v", owner, repo, pullRequest.GetNumber(), matched)

					_, err := ghClient.CreateComment(ctx, owner, repo, pullRequest.GetNumber(), commands.BuildWarningMessage(matched))
					if err != nil {
						log.Printf("[ERROR] failed to post warning comment: %v, owner %s, repo %s, pr %d", err, owner, repo, pullRequest.GetNumber())
					}
				}
			}
		}

		trigger, err := ghClient.FindTriggerComment(
			ctx,
			owner,
			repo,
			comments,
			false,
			pullRequest.GetNumber(),
			pullRequest.GetHTMLURL(),
		)
		if err != nil {
			log.Printf("[ERROR] failed to find trigger comment: %v, owner: %s, repo: %s, pr: %d", err, owner, repo, pullRequest.GetNumber())
			continue
		}

		// check auto trigger
		if trigger == nil && !pullRequest.GetDraft() {
			trigger = postAutoTrigger(ctx, owner, repo, ghClient, comments, pullRequest)
		}

		if trigger != nil {
			err = ghClient.AddCommentEyesReaction(ctx, owner, repo, trigger.CommentID)
			if err != nil {
				log.Printf("[ERROR] failed to add :eyes: reaction: %v, on owner: %s, repo: %s, pr: %d", err, owner, repo, pullRequest.GetNumber())
				continue
			}

			dispatchTrigger(trigger)
		}
	}
}

func pollIssues(
	ctx context.Context,
	owner, repo string,
	ghClient *github.Client,
	dispatchTrigger func(*github.Trigger),
) {
	issues, err := ghClient.GetIssuesWithLabel(ctx, owner, repo, "che-ai-assistant")
	if err != nil {
		log.Printf("[ERROR] failed to fetch issues: %v, owner %s, repo %s", err, owner, repo)
		return
	}

	for _, issue := range issues {
		comments, err := ghClient.GetComments(ctx, owner, repo, issue.GetNumber())
		if err != nil {
			log.Printf("[ERROR] failed to fetch comments: %v, owner %s, repo %s, issue %d", err, owner, repo, issue.GetNumber())
			continue
		}

		// post welcome message for issues
		if ghClient.IsIssueAuthorEligible(issue) && !ghClient.HasWelcomeComment(comments) {
			log.Printf("[INFO] posting issue welcome comment on %s/%s#%d", owner, repo, issue.GetNumber())

			err := ghClient.PostWelcomeComment(ctx, owner, repo, true, issue.GetNumber())
			if err != nil {
				log.Printf("[ERROR] failed to post issue welcome comment: %v, owner %s, repo %s, issue %d", err, owner, repo, issue.GetNumber())
			}
		}

		trigger, err := ghClient.FindTriggerComment(
			ctx,
			owner,
			repo,
			comments,
			true,
			issue.GetNumber(),
			issue.GetHTMLURL(),
		)
		if err != nil {
			log.Printf("[ERROR] failed to find trigger comment: %v, owner: %s, repo: %s, issue: %d", err, owner, repo, issue.GetNumber())
			continue
		}

		if trigger != nil {
			err = ghClient.AddCommentEyesReaction(
				ctx,
				owner,
				repo,
				trigger.CommentID,
			)
			if err != nil {
				log.Printf("[ERROR] failed to add :eyes: reaction: %v, on owner: %s, repo: %s, issue: %d", err, owner, repo, issue.GetNumber())
				continue
			}

			dispatchTrigger(trigger)
		}
	}
}

func postAutoTrigger(
	ctx context.Context,
	owner string,
	repo string,
	ghClient *github.Client,
	comments []*gh.IssueComment,
	pullRequest *gh.PullRequest,
) *github.Trigger {
	repoFullName := owner + "/" + repo
	for _, subCommand := range commands.SubCommands {
		if !subCommand.AutoTrigger {
			continue
		}

		if !commands.IsCommandAvailableForRepo(subCommand.Type, repoFullName) {
			continue
		}

		marker := commands.AutoTriggerMarker(subCommand.Type)
		if ghClient.HasAutoTriggerComment(comments, marker) {
			continue
		}

		passed, err := ghClient.AreCheckRunsPassed(ctx, owner, repo, pullRequest.GetHead().GetSHA())
		if err != nil {
			log.Printf("[ERROR] failed to check CI status: %v, owner: %s, repo: %s, pr: %d", err, owner, repo, pullRequest.GetNumber())
			continue
		}
		if !passed {
			continue
		}

		log.Printf("[INFO] auto-triggering %s on %s/%s#%d", subCommand.Type, owner, repo, pullRequest.GetNumber())

		comment, err := ghClient.CreateComment(ctx, owner, repo, pullRequest.GetNumber(), commands.BuildAutoTriggerComment(subCommand.Type))
		if err != nil {
			log.Printf("[ERROR] failed to post auto-trigger comment: %v, owner: %s, repo: %s, pr: %d", err, owner, repo, pullRequest.GetNumber())
			continue
		}

		return &github.Trigger{
			Owner:          owner,
			Repo:           repo,
			CommentID:      comment.GetID(),
			IsIssue:        false,
			IssueNumber:    pullRequest.GetNumber(),
			IssueURL:       pullRequest.GetHTMLURL(),
			CommentBody:    comment.GetBody(),
			SubCommandType: subCommand.Type,
		}
	}

	return nil
}

func parseRepoSlug(repo string) (owner, name string) {
	m := githubRepository.FindStringSubmatch(repo)
	if m == nil {
		return "", ""
	}

	return m[1], m[2]
}
