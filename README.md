# che-ai-assistant

A GitHub bot that monitors pull requests and issues, executing AI-powered tasks triggered by comments. Currently supports automatic documentation generation, PR reviews, and issue implementation for [Eclipse Che](https://github.com/eclipse-che) using Claude AI.

## How It Works

### Pull Requests

1. The bot polls configured GitHub repositories for open pull requests.
2. On eligible PRs, it posts a welcome comment listing available PR commands.
3. When an allowed user comments `/che-ai-assistant <command>`, the bot picks it up on the next poll cycle.
4. The bot marks the comment with a :eyes: reaction (to avoid reprocessing) and runs the corresponding handler.
5. Results are posted back to the PR as a comment update.

### Issues

1. The bot polls for open issues labeled `che-ai-assistant`.
2. On eligible issues, it posts a welcome comment listing available issue commands.
3. When an allowed user comments `/che-ai-assistant <command>`, the bot picks it up on the next poll cycle.
4. The bot marks the comment with a :eyes: reaction and runs the corresponding handler.

## Available Commands

### Pull Request Commands

| Command | Description |
|---------|-------------|
| `/che-ai-assistant generate-che-doc` | Generate a documentation PR based on the PR's changes |
| `/che-ai-assistant ok-pr-review` | Run a comprehensive PR review (summary, code review, deep review, impact analysis) |
| `/che-ai-assistant ok-pr-readiness` | Ensure PR has validation steps (only for `devfile/devworkspace-operator`, auto-triggered) |
| `/che-ai-assistant check-pr-test-failures` | Analyze failing CI checks, identify root causes, and suggest fixes |
| `/che-ai-assistant update-che-e2e-tests` | Update Eclipse Che e2e tests |
| `/che-ai-assistant claude <instruction>` | Run a free-form instruction on this PR |
| `/che-ai-assistant help` | Show available commands |

### Issue Commands

| Command | Description |
|---------|-------------|
| `/che-ai-assistant implement` | Implement a feature or fix a bug based on the issue description |
| `/che-ai-assistant help` | Show available commands |

## Prerequisites

- Go 1.25+
- A GitHub token with access to the watched repositories
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) installed (used by handlers that invoke Claude)

## Configuration

All configuration is via environment variables.

### Required

| Variable                                | Description                                                                                    |
|-----------------------------------------|------------------------------------------------------------------------------------------------|
| `CHE_AI_ASSISTANT_GITHUB_TOKEN`         | GitHub API token for the bot                                                                   |
| `CHE_AI_ASSISTANT_GITHUB_WATCH_REPOS`   | Comma-separated list of repositories to watch (e.g., `eclipse-che/che-server,eclipse-che/che`) |
| `CHE_AI_ASSISTANT_GITHUB_ALLOWED_USERS` | Comma-separated list of GitHub usernames authorized to trigger commands                        |
| `CHE_AI_MCP_SERVER_URL`                 | MCP server URL                                                                                 |

### Optional

| Variable | Default                     | Description |
|----------|-----------------------------|-------------|
| `CHE_AI_ASSISTANT_POLL_INTERVAL` | `5m`                        | How often to poll for new PR comments |
| `CHE_AI_ASSISTANT_TASK_TIMEOUT` | `30m`                       | Maximum time a handler can run |
| `CHE_AI_ASSISTANT_MAX_CONCURRENT` | `1`                         | Maximum number of handlers running concurrently |
| `CHE_AI_ASSISTANT_TEMPLATES_DIR` | `templates`                 | Directory containing prompt templates |
| `CHE_AI_ASSISTANT_OUTPUT_DIR` | System temp dir             | Directory for output files |
| `CHE_AI_ASSISTANT_LOG_FILE` | `<tmp>/che-ai-assistant.log` | Log file path |
| `CHE_DELETE_DEV_WORKSPACE` | `true`                      | Whether to delete DevWorkspaces after task completion |
| `CHE_AI_ASSISTANT_WARN_DIRS_COMMITS` | `.claude,.vscode`           | Comma-separated directories to warn about in PR commits |

## Build and Run

```bash
# Build
make build

# Run tests
make test

# Lint
make lint
```

## Adding a New Handler

For most commands, adding a new subcommand requires just two steps: define the subcommand and create a prompt template. The `processDefault` handler in `pkg/processor/processor.go` automatically picks up any command that has a matching template.

### 1. Define the Subcommand

In `pkg/commands/commands.go`, add a new `SubCommandType` constant and register it in the `SubCommands` slice:

```go
const (
    SubCommandGenerateCheDoc SubCommandType = "generate-che-doc"
    SubCommandMyCommand      SubCommandType = "my-command"       // add this
    SubCommandHelp           SubCommandType = "help"
)

var SubCommands = []SubCommand{
    {Type: SubCommandGenerateCheDoc, Description: "Generate a documentation PR based on this PR's changes"},
    {Type: SubCommandMyCommand, Description: "Short description of what it does", IssueOnly: true},  // add this
    {Type: SubCommandHelp, Description: "Show this help message"},
}
```

The `SubCommand` struct supports these fields:

| Field | Description |
|-------|-------------|
| `IssueOnly` | When `true`, the command is only available on issues (triggered via the `che-ai-assistant` label). Commands without this flag are available on pull requests only. |
| `AllowedRepos` | Restricts the command to specific repositories (e.g., `[]string{"devfile/devworkspace-operator"}`). Empty means available everywhere. |
| `AutoTrigger` | When `true`, the command is automatically triggered on eligible PRs without a user comment. |

### 2. Create a Prompt Template

Create `templates/my-command.tmpl`. The template is a Go text/template that receives the following variables:

| Variable | Description |
|----------|-------------|
| `{{.PullRequestURL}}` | The HTML URL of the triggering pull request (same as `IssueURL`) |
| `{{.IssueURL}}` | The HTML URL of the triggering issue or pull request |
| `{{.Args}}` | Any text after the subcommand (e.g., the instruction in `/che-ai-assistant claude <instruction>`) |

```
You are an automated assistant. Follow these steps exactly:
1. ...
2. Use information from this issue: {{.IssueURL}}
3. ...
```

Requirements:
- The filename (minus `.tmpl`) must match the `SubCommandType` constant value.

### 3. Custom Processing (optional)

If your command needs special processing beyond the default template-based flow, add a case to the `switch` statement in `pkg/processor/processor.go`'s `Trigger` method.

> **Note:** For issue-only commands, the bot will only pick up the command from issues labeled `che-ai-assistant`. For PR commands, the bot monitors all open pull requests from allowed users.

## License

[Eclipse Public License 2.0](https://www.eclipse.org/legal/epl-2.0/)
