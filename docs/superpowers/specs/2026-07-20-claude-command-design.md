# Design: `claude` command

Add a new `claude` subcommand that accepts free-form user instructions as a parameter, available for all repos, PR-only.

Usage: `/che-ai-assistant claude <instruction>`

## Changes

### 1. Data model

**`commands.go`** — new constant:

```go
SubCommandClaude SubCommandType = "claude"
```

Registered in `SubCommands`:

```go
{
    Type:        SubCommandClaude,
    Description: "Run a free-form instruction on this PR",
}
```

No `IssueOnly`, no `AllowedRepos` — available for all repos, PR-only by default (existing `FindTriggerComment` logic skips non-`IssueOnly` commands on issues).

**`github/client.go`** — add `Args string` field to `Trigger` struct. Populated from the third return value of `Parse`.

### 2. Parsing

**`Parse` signature change**:

```go
func Parse(body string) (bool, SubCommandType, string)
```

Third return value is everything after the subcommand name, trimmed. For `/che-ai-assistant claude fix the typo`, returns `("claude", "fix the typo")`. For commands without args, returns empty string.

Update callers:
- `FindTriggerComment` — stores args in `Trigger.Args`
- `commands_test.go` — update test assertions for third return value

### 3. Prompt building

**`buildPrompt`** — add `Args` to the template data map:

```go
data := map[string]string{
    "PullRequestURL": trigger.IssueURL,
    "IssueURL":       trigger.IssueURL,
    "Args":           trigger.Args,
}
```

**Validation** — `processDefault` checks that `Args` is non-empty for the `claude` command before proceeding. If empty, posts an error message to the PR comment.

### 4. Template

New file `templates/claude.tmpl`:

```
IMPORTANT: You are running unattended. Do NOT use AskUserQuestion at any point.
Skip any user confirmation steps and proceed with your best suggestions automatically.

Pull request: {{.PullRequestURL}}

Task: {{.Args}}
```

### 5. Processing flow

The `claude` command flows through `processDefault` — no custom handler. The only addition is the empty-args validation check before `buildPrompt`.

## Files modified

- `pkg/commands/commands.go` — new constant, registration, `Parse` signature change
- `pkg/commands/commands_test.go` — update tests for new `Parse` signature, add tests for args extraction
- `pkg/github/client.go` — `Args` field on `Trigger`, pass through from `Parse`
- `pkg/processor/processor.go` — pass `Args` to template data, add validation
- `templates/claude.tmpl` — new template file
