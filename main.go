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

	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/github"
	"github.com/tolusha/che-doc-generator/pkg/processor"
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

	return func() {
		for _, repositoryUrl := range cfg.GitHubWatchRepos {
			owner, repo := parseRepoSlug(repositoryUrl)
			if owner == "" || repo == "" {
				log.Printf("[ERROR] invalid repo format: %s (expected owner/repo or https://github.com/owner/repo)", repositoryUrl)
				continue
			}

			pullRequests, err := ghClient.GetPullRequests(ctx, owner, repo)
			if err != nil {
				log.Printf("[ERROR] failed to fetch pull requests: %v, owner %s, repo %s", err, owner, repo)
				continue
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

					err := ghClient.PostWelcomeComment(ctx, owner, repo, pullRequest)
					if err != nil {
						log.Printf("[ERROR] failed to post welcome comment: %v, owner %s, repo %s, pr %d", err, owner, repo, pullRequest.GetNumber())
					}
				}

				trigger, err := ghClient.FindTriggerComment(ctx, owner, repo, comments, pullRequest)
				if err != nil {
					log.Printf("[ERROR] failed to find trigger comment: %v, owner: %s, repo: %s, pr: %d", err, owner, repo, pullRequest.GetNumber())
					continue
				}

				if trigger == nil {
					continue
				}

				err = ghClient.AddIssueCommentEyesReaction(ctx, owner, repo, trigger.CommentID)
				if err != nil {
					log.Printf("[ERROR] failed to add :eyes: reaction: %v, on owner: %s, repo: %s, pr: %d", err, owner, repo, pullRequest.GetNumber())
					continue
				}

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
		}
	}
}

func parseRepoSlug(repo string) (owner, name string) {
	m := githubRepository.FindStringSubmatch(repo)
	if m == nil {
		return "", ""
	}

	return m[1], m[2]
}
