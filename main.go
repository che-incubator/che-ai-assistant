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
	"che-incubator/che-ai-assistant/pkg/commands"
	"che-incubator/che-ai-assistant/pkg/config"
	"che-incubator/che-ai-assistant/pkg/github"
	"che-incubator/che-ai-assistant/pkg/processor"
	"che-incubator/che-ai-assistant/pkg/scanner"
	"che-incubator/che-ai-assistant/pkg/state"
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
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

	store, err := state.NewStore(cfg.StateFile)
	if err != nil {
		log.Fatalf("[ERROR] state.NewStore: %v", err)
	}

	taskProcessor, err := processor.NewTaskProcessor(cfg, ghClient)
	if err != nil {
		log.Fatalf("[ERROR] processor.NewTaskProcessor: %v", err)
	}

	log.Printf("[INFO] starting che-ai-assistant: repositories %v, poll every %v", cfg.GitHubRepositories, cfg.GitHubPollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	var wg sync.WaitGroup

	poll := pollFunc(ctx, &wg, cfg, ghClient, taskProcessor, store)

	ticker := time.NewTicker(cfg.GitHubPollInterval)
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
	store *state.Store,
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
		cleanupClosedEntries(ctx, ghClient, store)

		for _, repositoryUrl := range cfg.GitHubRepositories {
			owner, repo := github.ParseRepoSlug(repositoryUrl)
			if owner == "" || repo == "" {
				log.Printf("[ERROR] invalid repo format: %s (expected owner/repo or https://github.com/owner/repo)", repositoryUrl)
				continue
			}

			pollPullRequests(ctx, owner, repo, cfg, ghClient, store, dispatchTrigger, store.GetStartTime())
			pollIssues(ctx, owner, repo, ghClient, store, dispatchTrigger, store.GetStartTime())
		}
	}
}

func pollPullRequests(
	ctx context.Context,
	owner, repo string,
	cfg *config.Config,
	ghClient *github.Client,
	store *state.Store,
	dispatchTrigger func(*github.Trigger),
	startTime *time.Time,
) {
	pullRequests, err := ghClient.GetPullRequests(ctx, owner, repo)
	if err != nil {
		log.Printf("[ERROR] failed to fetch pull requests: %v, https://github.com/%s/%s", err, owner, repo)
		return
	}

	for _, pullRequest := range pullRequests {
		prURL := pullRequest.GetHTMLURL()

		comments, err := ghClient.GetComments(ctx, owner, repo, *pullRequest.Number)
		if err != nil {
			log.Printf("[ERROR] failed to fetch comments: %v, %s", err, prURL)
			continue
		}

		// post welcome message
		if ghClient.IsPullRequestAuthorEligible(pullRequest) && !ghClient.HasWelcomeComment(comments) {
			log.Printf("[INFO] posting welcome comment on %s", prURL)

			err := ghClient.PostWelcomeComment(ctx, owner, repo, false, pullRequest.GetNumber())
			if err != nil {
				log.Printf("[ERROR] failed to post welcome comment: %v, %s", err, prURL)
			}
		}

		// post warning message
		if !ghClient.HasWarningComment(comments) {
			files, err := ghClient.GetPullRequestFiles(ctx, owner, repo, pullRequest.GetNumber())
			if err != nil {
				log.Printf("[ERROR] failed to fetch pull request files: %v, %s", err, prURL)
			} else {
				matched := scanner.CheckFiles(files, cfg.WarnDirsCommits)
				if len(matched) > 0 {
					log.Printf("[INFO] posting warning comment on %s for files: %v", prURL, matched)

					_, err := ghClient.CreateComment(ctx, owner, repo, pullRequest.GetNumber(), commands.BuildWarningMessage(matched))
					if err != nil {
						log.Printf("[ERROR] failed to post warning comment: %v, %s", err, prURL)
					}
				}
			}
		}

		isProcessed := func(commentID int64) bool {
			return store.IsProcessed(owner, repo, pullRequest.GetNumber(), commentID)
		}

		trigger, err := ghClient.FindTriggerComment(
			ctx,
			comments,
			false,
			pullRequest.GetNumber(),
			pullRequest.GetHTMLURL(),
			owner,
			repo,
			isProcessed,
			startTime,
		)
		if err != nil {
			log.Printf("[ERROR] failed to find trigger comment: %v, %s", err, prURL)
			continue
		}

		if trigger != nil {
			if err := store.MarkProcessed(owner, repo, pullRequest.GetNumber(), trigger.CommentID); err != nil {
				log.Printf("[ERROR] failed to mark trigger as processed: %v, %s", err, prURL)
				continue
			}

			err = ghClient.AddCommentEyesReaction(ctx, owner, repo, trigger.CommentID)
			if err != nil {
				log.Printf("[ERROR] failed to add :eyes: reaction: %v, %s", err, prURL)
			}

			dispatchTrigger(trigger)
		}
	}
}

func pollIssues(
	ctx context.Context,
	owner, repo string,
	ghClient *github.Client,
	store *state.Store,
	dispatchTrigger func(*github.Trigger),
	startTime *time.Time,
) {
	issues, err := ghClient.GetIssuesWithLabel(ctx, owner, repo, "che-ai-assistant")
	if err != nil {
		log.Printf("[ERROR] failed to fetch issues: %v, https://github.com/%s/%s", err, owner, repo)
		return
	}

	for _, issue := range issues {
		issueURL := issue.GetHTMLURL()

		comments, err := ghClient.GetComments(ctx, owner, repo, issue.GetNumber())
		if err != nil {
			log.Printf("[ERROR] failed to fetch comments: %v, %s", err, issueURL)
			continue
		}

		// post welcome message for issues
		if ghClient.IsIssueAuthorEligible(issue) && !ghClient.HasWelcomeComment(comments) {
			log.Printf("[INFO] posting issue welcome comment on %s", issueURL)

			err := ghClient.PostWelcomeComment(ctx, owner, repo, true, issue.GetNumber())
			if err != nil {
				log.Printf("[ERROR] failed to post issue welcome comment: %v, %s", err, issueURL)
			}
		}

		isProcessed := func(commentID int64) bool {
			return store.IsProcessed(owner, repo, issue.GetNumber(), commentID)
		}

		trigger, err := ghClient.FindTriggerComment(
			ctx,
			comments,
			true,
			issue.GetNumber(),
			issue.GetHTMLURL(),
			owner,
			repo,
			isProcessed,
			startTime,
		)
		if err != nil {
			log.Printf("[ERROR] failed to find trigger comment: %v, %s", err, issueURL)
			continue
		}

		if trigger != nil {
			if err := store.MarkProcessed(owner, repo, issue.GetNumber(), trigger.CommentID); err != nil {
				log.Printf("[ERROR] failed to mark trigger as processed: %v, %s", err, issueURL)
				continue
			}

			err = ghClient.AddCommentEyesReaction(
				ctx,
				owner,
				repo,
				trigger.CommentID,
			)
			if err != nil {
				log.Printf("[ERROR] failed to add :eyes: reaction: %v, %s", err, issueURL)
			}

			dispatchTrigger(trigger)
		}
	}
}

func cleanupClosedEntries(ctx context.Context, ghClient *github.Client, store *state.Store) {
	for _, key := range store.GetOpenKeys() {
		owner, repo, number, err := state.ParseKey(key)
		if err != nil {
			log.Printf("[WARN] invalid state key %q, removing: %v", key, err)
			if err := store.RemoveKey(key); err != nil {
				log.Printf("[ERROR] failed to remove invalid state key %q: %v", key, err)
			}
			continue
		}

		closed, err := ghClient.IsIssueClosed(ctx, owner, repo, number)
		if err != nil {
			log.Printf("[ERROR] failed to check issue state for %s: %v", key, err)
			continue
		}

		if closed {
			log.Printf("[INFO] removing state for closed %s", key)
			if err := store.RemoveKey(key); err != nil {
				log.Printf("[ERROR] failed to remove state for %s: %v", key, err)
			}
		}
	}
}
