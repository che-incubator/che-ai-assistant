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

package github

import (
	"che-incubator/che-ai-assistant/pkg/commands"
	"che-incubator/che-ai-assistant/pkg/config"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

type Trigger struct {
	Owner          string
	Repo           string
	IsIssue        bool
	IssueNumber    int
	IssueURL       string
	CommentID      int64
	CommentBody    string
	SubCommandType commands.SubCommandType
	Args           string
	User           string
}

type Client struct {
	client       *github.Client
	allowedUsers []string
	pollInterval time.Duration
}

const (
	eyesReaction = "eyes"
)

func NewGitHubClient(cfg *config.Config) *Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.GitHubToken})
	httpClient := oauth2.NewClient(context.Background(), tokenSource)
	client := github.NewClient(httpClient)

	return &Client{
		client:       client,
		allowedUsers: cfg.GitHubAllowedUsers,
		pollInterval: cfg.TasksPollInterval,
	}
}

func (g *Client) FindTriggerComment(
	ctx context.Context,
	comments []*github.IssueComment,
	isIssue bool,
	issueNumber int,
	issueURL string,
	owner, repo string,
	isProcessed func(commentID int64) bool,
) (*Trigger, error) {
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]

		ok, subCommand, args := commands.Parse(comment.GetBody())
		if !ok {
			continue
		}

		if isIssue != commands.IsIssueOnlyCommand(subCommand) {
			continue
		}

		if !g.IsCommentAuthorEligible(comment) {
			continue
		}

		hasEyeReaction, err := g.HasCommentEyesReaction(ctx, owner, repo, comment.GetID())
		if err != nil {
			return nil, err
		}

		// for backward compatability
		if hasEyeReaction {
			break
		}

		if isProcessed(comment.GetID()) {
			continue
		}

		return &Trigger{
			Owner:          owner,
			Repo:           repo,
			CommentID:      comment.GetID(),
			IsIssue:        isIssue,
			IssueNumber:    issueNumber,
			IssueURL:       issueURL,
			CommentBody:    comment.GetBody(),
			SubCommandType: subCommand,
			Args:           args,
			User:           comment.GetUser().GetLogin(),
		}, nil
	}

	return nil, nil
}

func (g *Client) GetPullRequests(
	ctx context.Context,
	owner, repo string,
) ([]*github.PullRequest, error) {

	var result []*github.PullRequest

	opts := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		pullRequests, response, err := g.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}

		result = append(result, pullRequests...)

		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	return result, nil
}

func (g *Client) GetComments(
	ctx context.Context,
	owner, repo string,
	issueNumber int,
) ([]*github.IssueComment, error) {

	var result []*github.IssueComment

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := g.client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
		if err != nil {
			return nil, err
		}

		result = append(result, comments...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return result, nil
}

func (g *Client) PostWelcomeComment(
	ctx context.Context,
	owner, repo string,
	isIssue bool,
	issueNumber int,
) error {
	var welcomeMessage string
	if isIssue {
		welcomeMessage = commands.BuildIssueWelcomeMessage(owner+"/"+repo, g.pollInterval)
	} else {
		welcomeMessage = commands.BuildPRWelcomeMessage(owner+"/"+repo, g.pollInterval)
	}

	_, err := g.CreateComment(
		ctx,
		owner,
		repo,
		issueNumber,
		welcomeMessage,
	)

	return err
}

func (g *Client) IsPullRequestAuthorEligible(pullRequest *github.PullRequest) bool {
	return slices.Contains(g.allowedUsers, pullRequest.GetUser().GetLogin())
}

func (g *Client) IsIssueAuthorEligible(issue *github.Issue) bool {
	return slices.Contains(g.allowedUsers, issue.GetUser().GetLogin())
}

func (g *Client) IsCommentAuthorEligible(comment *github.IssueComment) bool {
	return slices.Contains(g.allowedUsers, comment.GetUser().GetLogin())
}

func (g *Client) HasWelcomeComment(comments []*github.IssueComment) bool {
	return slices.ContainsFunc(comments, func(c *github.IssueComment) bool {
		return strings.Contains(c.GetBody(), commands.WelcomeMarker)
	})
}

func (g *Client) HasWarningComment(comments []*github.IssueComment) bool {
	return slices.ContainsFunc(comments, func(c *github.IssueComment) bool {
		return strings.Contains(c.GetBody(), commands.WarningMarker)
	})
}

func (g *Client) HasAutoTriggerComment(comments []*github.IssueComment, marker string) bool {
	return slices.ContainsFunc(comments, func(c *github.IssueComment) bool {
		return strings.Contains(c.GetBody(), marker)
	})
}

func (g *Client) AreCheckRunsPassed(
	ctx context.Context,
	owner, repo, ref string,
) (bool, error) {
	opts := &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allRuns []*github.CheckRun
	for {
		result, resp, err := g.client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opts)
		if err != nil {
			return false, err
		}

		allRuns = append(allRuns, result.CheckRuns...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if len(allRuns) == 0 {
		return false, nil
	}

	for _, run := range allRuns {
		if run.GetStatus() != "completed" {
			return false, nil
		}

		conclusion := run.GetConclusion()
		if conclusion != "success" && conclusion != "neutral" && conclusion != "skipped" {
			return false, nil
		}
	}

	return true, nil
}

func (g *Client) CreateComment(
	ctx context.Context,
	owner, repo string,
	issueNumber int,
	body string,
) (*github.IssueComment, error) {
	comment, _, err := g.client.Issues.CreateComment(
		ctx,
		owner,
		repo,
		issueNumber,
		&github.IssueComment{
			Body: github.Ptr(body),
		},
	)

	return comment, err
}

func (g *Client) UpdateComment(
	ctx context.Context,
	owner, repo string,
	commentId int64,
	body string,
) error {
	_, _, err := g.client.Issues.EditComment(
		ctx,
		owner,
		repo,
		commentId,
		&github.IssueComment{
			Body: github.Ptr(body),
		},
	)

	return err
}

func (g *Client) AddCommentEyesReaction(
	ctx context.Context,
	owner, repo string,
	commentId int64,
) error {
	_, _, err := g.client.Reactions.CreateIssueCommentReaction(
		ctx,
		owner,
		repo,
		commentId,
		eyesReaction,
	)

	return err
}

func (g *Client) HasCommentEyesReaction(ctx context.Context,
	owner, repo string,
	commentId int64) (bool, error) {

	opts := &github.ListOptions{PerPage: 10}

	for {
		reactions, resp, err := g.client.Reactions.ListIssueCommentReactions(ctx, owner, repo, commentId, opts)
		if err != nil {
			return false, err
		}

		for _, r := range reactions {
			if r.GetContent() == eyesReaction {
				return true, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return false, nil
}

func (g *Client) IsIssueClosed(
	ctx context.Context,
	owner, repo string,
	number int,
) (bool, error) {
	issue, _, err := g.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return false, err
	}

	return issue.GetState() == "closed", nil
}

func (g *Client) GetIssuesWithLabel(
	ctx context.Context,
	owner, repo string,
	label string,
) ([]*github.Issue, error) {
	var result []*github.Issue

	opts := &github.IssueListByRepoOptions{
		State:       "open",
		Labels:      []string{label},
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		issues, resp, err := g.client.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}

		for _, issue := range issues {
			if issue.PullRequestLinks == nil {
				result = append(result, issue)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return result, nil
}

func (g *Client) GetPullRequestFiles(
	ctx context.Context,
	owner, repo string,
	pullRequestNumber int,
) ([]*github.CommitFile, error) {
	var result []*github.CommitFile

	opts := &github.ListOptions{PerPage: 100}

	for {
		files, resp, err := g.client.PullRequests.ListFiles(ctx, owner, repo, pullRequestNumber, opts)
		if err != nil {
			return nil, err
		}

		result = append(result, files...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return result, nil
}
