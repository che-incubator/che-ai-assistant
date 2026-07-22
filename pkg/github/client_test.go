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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v68/github"
)

func newTestClient(allowedUsers []string, baseURL string) *Client {
	c := gh.NewClient(nil)
	c.BaseURL, _ = c.BaseURL.Parse(baseURL + "/")

	return &Client{
		client:       c,
		allowedUsers: allowedUsers,
	}
}

func noReactionsServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/{owner}/{repo}/issues/comments/{id}/reactions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]*gh.Reaction{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func neverProcessed(_ int64) bool { return false }

func TestFindTriggerComment_FindsUnprocessed(t *testing.T) {
	srv := noReactionsServer(t)
	client := newTestClient([]string{"alice"}, srv.URL)

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
		{
			ID:   gh.Ptr(int64(101)),
			Body: gh.Ptr("just a regular comment"),
			User: &gh.User{Login: gh.Ptr("bob")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), comments, false, 1, "https://github.com/org/repo/pull/1", "org", "repo", neverProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger == nil {
		t.Fatal("expected a trigger, got nil")
	}
	if trigger.CommentID != 100 {
		t.Errorf("expected comment ID 100, got %d", trigger.CommentID)
	}
	if trigger.IssueNumber != 1 {
		t.Errorf("expected issue number 1, got %d", trigger.IssueNumber)
	}
	if trigger.CommentBody != "/che-ai-assistant generate-che-doc" {
		t.Errorf("expected comment body preserved, got %q", trigger.CommentBody)
	}
	if trigger.SubCommandType != commands.SubCommandGenerateCheDoc {
		t.Errorf("expected generate-che-doc command, got %q", trigger.SubCommandType)
	}
}

func TestFindTriggerComment_ParsesSubcommand(t *testing.T) {
	srv := noReactionsServer(t)
	client := newTestClient([]string{"alice"}, srv.URL)

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant help"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), comments, false, 1, "", "org", "repo", neverProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger == nil {
		t.Fatal("expected a trigger, got nil")
	}
	if trigger.SubCommandType != commands.SubCommandHelp {
		t.Errorf("expected help command, got %q", trigger.SubCommandType)
	}
}

func TestFindTriggerComment_SkipsProcessed(t *testing.T) {
	srv := noReactionsServer(t)
	client := newTestClient([]string{"alice"}, srv.URL)

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	alwaysProcessed := func(_ int64) bool { return true }

	trigger, err := client.FindTriggerComment(context.Background(), comments, false, 1, "", "org", "repo", alwaysProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger != nil {
		t.Fatalf("expected nil trigger (already processed), got %+v", trigger)
	}
}

func TestFindTriggerComment_SkipsUnauthorizedUser(t *testing.T) {
	client := newTestClient([]string{"alice", "bob"}, "http://unused")

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("mallory")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), comments, false, 1, "", "org", "repo", neverProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger != nil {
		t.Fatalf("expected nil trigger (unauthorized user), got %+v", trigger)
	}
}

func TestIsPullRequestAuthorEligible(t *testing.T) {
	client := newTestClient([]string{"alice", "bob"}, "http://unused")

	allowed := &gh.PullRequest{User: &gh.User{Login: gh.Ptr("alice")}}
	if !client.IsPullRequestAuthorEligible(allowed) {
		t.Error("expected alice to be eligible")
	}

	denied := &gh.PullRequest{User: &gh.User{Login: gh.Ptr("mallory")}}
	if client.IsPullRequestAuthorEligible(denied) {
		t.Error("expected mallory to not be eligible")
	}
}

func TestHasWelcomeComment(t *testing.T) {
	client := newTestClient(nil, "http://unused")

	withMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
		{Body: gh.Ptr(commands.BuildPRWelcomeMessage("test-org/test-repo"))},
	}
	if !client.HasWelcomeComment(withMarker) {
		t.Error("expected bot comment to be found")
	}

	withoutMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
	}
	if client.HasWelcomeComment(withoutMarker) {
		t.Error("expected no bot comment")
	}
}

func TestPostWelcomeComment(t *testing.T) {
	var posted bool
	mux := http.NewServeMux()

	mux.HandleFunc("POST /repos/org/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		posted = true
		var body gh.IssueComment
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.GetBody(), commands.WelcomeMarker) {
			t.Error("welcome comment should contain marker")
		}
		_ = json.NewEncoder(w).Encode(&gh.IssueComment{ID: gh.Ptr(int64(200))})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient([]string{"alice"}, srv.URL)
	err := client.PostWelcomeComment(context.Background(), "org", "repo", false, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !posted {
		t.Error("expected welcome comment to be posted")
	}
}

func TestUpdateComment(t *testing.T) {
	var updated bool
	mux := http.NewServeMux()

	mux.HandleFunc("PATCH /repos/org/repo/issues/comments/100", func(w http.ResponseWriter, r *http.Request) {
		updated = true
		var body gh.IssueComment
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.GetBody(), "Documentation PR created") {
			t.Error("updated body should contain new content")
		}
		_ = json.NewEncoder(w).Encode(&gh.IssueComment{ID: gh.Ptr(int64(100))})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)
	err := client.UpdateComment(context.Background(), "org", "repo", 100, "Documentation PR created: https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected comment to be updated")
	}
}

func TestHasWarningComment(t *testing.T) {
	client := newTestClient(nil, "http://unused")

	withMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
		{Body: gh.Ptr(commands.WarningMarker + "\n⚠️ **Warning**")},
	}
	if !client.HasWarningComment(withMarker) {
		t.Error("expected warning comment to be found")
	}

	withoutMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
	}
	if client.HasWarningComment(withoutMarker) {
		t.Error("expected no warning comment")
	}
}

func TestHasAutoTriggerComment(t *testing.T) {
	client := newTestClient(nil, "http://unused")
	marker := commands.AutoTriggerMarker(commands.SubCommandPullRequestReadiness)

	withMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
		{Body: gh.Ptr(commands.BuildAutoTriggerComment(commands.SubCommandPullRequestReadiness))},
	}
	if !client.HasAutoTriggerComment(withMarker, marker) {
		t.Error("expected auto-trigger comment to be found")
	}

	withoutMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
	}
	if client.HasAutoTriggerComment(withoutMarker, marker) {
		t.Error("expected no auto-trigger comment")
	}
}

func TestAreCheckRunsPassed_AllSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/org/repo/commits/abc123/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(gh.ListCheckRunsResults{
			Total: gh.Ptr(2),
			CheckRuns: []*gh.CheckRun{
				{Status: gh.Ptr("completed"), Conclusion: gh.Ptr("success")},
				{Status: gh.Ptr("completed"), Conclusion: gh.Ptr("success")},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)
	passed, err := client.AreCheckRunsPassed(context.Background(), "org", "repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("expected checks to pass")
	}
}

func TestAreCheckRunsPassed_SomePending(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/org/repo/commits/abc123/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(gh.ListCheckRunsResults{
			Total: gh.Ptr(2),
			CheckRuns: []*gh.CheckRun{
				{Status: gh.Ptr("completed"), Conclusion: gh.Ptr("success")},
				{Status: gh.Ptr("in_progress"), Conclusion: nil},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)
	passed, err := client.AreCheckRunsPassed(context.Background(), "org", "repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected checks not to pass when some are pending")
	}
}

func TestAreCheckRunsPassed_SomeFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/org/repo/commits/abc123/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(gh.ListCheckRunsResults{
			Total: gh.Ptr(2),
			CheckRuns: []*gh.CheckRun{
				{Status: gh.Ptr("completed"), Conclusion: gh.Ptr("success")},
				{Status: gh.Ptr("completed"), Conclusion: gh.Ptr("failure")},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)
	passed, err := client.AreCheckRunsPassed(context.Background(), "org", "repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected checks not to pass when some failed")
	}
}

func TestAreCheckRunsPassed_NoCheckRuns(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/org/repo/commits/abc123/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(gh.ListCheckRunsResults{
			Total:     gh.Ptr(0),
			CheckRuns: []*gh.CheckRun{},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)
	passed, err := client.AreCheckRunsPassed(context.Background(), "org", "repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Error("expected checks not to pass when there are no check runs")
	}
}

func TestFindTriggerComment_SkipsIssueOnlyCommand(t *testing.T) {
	client := newTestClient([]string{"alice"}, "http://unused")

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant implement"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), comments, false, 1, "", "org", "repo", neverProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger != nil {
		t.Fatalf("expected nil trigger (issue-only command on PR), got %+v", trigger)
	}
}

func TestFindTriggerComment_FindsImplementForIssue(t *testing.T) {
	srv := noReactionsServer(t)
	client := newTestClient([]string{"alice"}, srv.URL)

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant implement"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), comments, true, 42, "https://github.com/org/repo/issues/42", "org", "repo", neverProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger == nil {
		t.Fatal("expected a trigger, got nil")
	}
	if trigger.IssueNumber != 42 {
		t.Errorf("expected issue number 42, got %d", trigger.IssueNumber)
	}
	if trigger.IssueURL != "https://github.com/org/repo/issues/42" {
		t.Errorf("expected issue URL, got %q", trigger.IssueURL)
	}
	if !trigger.IsIssue {
		t.Error("expected IsIssue to be true")
	}
	if trigger.SubCommandType != commands.SubCommandImplement {
		t.Errorf("expected implement command, got %q", trigger.SubCommandType)
	}
}

func TestFindTriggerComment_SkipsPROnlyCommandForIssue(t *testing.T) {
	client := newTestClient([]string{"alice"}, "http://unused")

	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/che-ai-assistant generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), comments, true, 42, "https://github.com/org/repo/issues/42", "org", "repo", neverProcessed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger != nil {
		t.Fatalf("expected nil trigger (PR-only command on issue), got %+v", trigger)
	}
}

func TestGetPullRequestFiles(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")

		if page == "" || page == "1" {
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]*gh.CommitFile{
				{Filename: gh.Ptr(".claude/settings.json")},
				{Filename: gh.Ptr("main.go")},
			})
		} else {
			_ = json.NewEncoder(w).Encode([]*gh.CommitFile{
				{Filename: gh.Ptr(".vscode/launch.json")},
			})
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)

	files, err := client.GetPullRequestFiles(context.Background(), "org", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0].GetFilename() != ".claude/settings.json" {
		t.Errorf("expected first file .claude/settings.json, got %s", files[0].GetFilename())
	}
	if files[2].GetFilename() != ".vscode/launch.json" {
		t.Errorf("expected third file .vscode/launch.json, got %s", files[2].GetFilename())
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (pagination), got %d", callCount)
	}
}
