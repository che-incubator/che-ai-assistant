# ok-pr-review Handler Design

## Overview

Wire up the `ok-pr-review` command so that when a user comments `/che-ai-assistant ok-pr-review` on a PR, the bot runs five `ok-pr-review` skills in a DevWorkspace and posts the results back as PR comments.

## Skills and Execution Order

1. **`/ok-pr-review:learn-repo`** — builds repo context (output NOT posted)
2. **`/ok-pr-review:summary`** — quick PR summary with risks (posted)
3. **`/ok-pr-review:review`** — standard code review (posted)
4. **`/ok-pr-review:deep-review`** — design quality, anti-patterns, testing rigor (posted)
5. **`/ok-pr-review:impact`** — system-level review: supply chain, RBAC, ops, compatibility (posted)

## Architecture

Follows the same DevWorkspace pattern as `generate-che-doc`:

```
User comments on PR
  → Bot picks up trigger
    → Handler.Run() executes outer Claude with prompt template
      → Outer Claude creates DevWorkspace via che-mcp-server
        → Injects Claude into DevWorkspace
          → Inner Claude runs 5 skills sequentially
          → Returns 4 delimited review outputs
        → Outer Claude deletes DevWorkspace
      → Returns delimited output
    → Handler parses output into 4 segments
  → Handler.OnSuccess() posts each segment as a separate PR comment
```

## Template (`templates/ok-pr-review.tmpl`)

The prompt template instructs the outer Claude to:

1. Create a DevWorkspace via `che-mcp-server` with 4Gi memory limit. The DevWorkspace name must include the repository name and PR number (e.g., `ok-pr-review-repo-pr-123`).
2. Inject Claude into the DevWorkspace.
3. In the DevWorkspace:
   - Authenticate: `echo $CHE_AI_ASSISTANT_OK_PR_REVIEW_GITHUB_TOKEN | gh auth login --with-token`
   - Run `/ok-pr-review:learn-repo` on the target repository (output discarded — this is for context building only).
   - Run each of the following four skills on the target PR, outputting the full result of each followed by the delimiter `===OK-PR-REVIEW-OUTPUT===` on its own line:
     1. `/ok-pr-review:summary`
     2. `/ok-pr-review:review`
     3. `/ok-pr-review:deep-review`
     4. `/ok-pr-review:impact`
   - The bot is running unattended: do NOT use `AskUserQuestion`, skip any user confirmation steps.
4. Delete the DevWorkspace.
5. Return ONLY the concatenated delimited output.

Template placeholder: `{{.PRURL}}` — the HTML URL of the triggering PR.

## Handler (`pkg/processor/handlers/ok_pr_review_handler.go`)

### `Run` method

1. Execute `claude --dangerously-skip-permissions -p <prompt> --output-format json`.
2. Parse the raw output as JSON to extract the `result` field using:
   ```go
   type claudeOutput struct {
       Result string `json:"result"`
   }
   ```
3. Split `result` by `===OK-PR-REVIEW-OUTPUT===`.
4. Trim whitespace and filter out empty segments.
5. Return the non-empty segments as `[]string`.
6. If fewer than 4 segments are found, return what is available (partial results are still useful) rather than erroring.

### `OnSuccess` method

No changes needed. Already iterates over `outputs` and posts each as a separate PR comment.

### `OnFailure` method

No changes needed. Already appends a failure message to the original comment.

## Authentication

Uses a dedicated environment variable: `CHE_AI_ASSISTANT_OK_PR_REVIEW_GITHUB_TOKEN`. This token needs read access to the target repository for the review skills to fetch PR diffs and file contents.

## Output Delimiter

`===OK-PR-REVIEW-OUTPUT===` — placed on its own line after each of the four review skill outputs. Chosen to be unique enough to avoid collision with review content.

## Files Changed

| File | Change |
|------|--------|
| `templates/ok-pr-review.tmpl` | Rewrite with full DevWorkspace + 5-skill execution instructions |
| `pkg/processor/handlers/ok_pr_review_handler.go` | Add JSON parsing, delimiter splitting in `Run` |
