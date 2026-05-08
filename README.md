# che-ai-pullrequest-assistant

A GitHub bot that monitors pull requests and executes AI-powered tasks triggered by PR comments. Currently supports automatic documentation generation for [Eclipse Che](https://github.com/eclipse-che) using Claude AI.

## How It Works

1. The bot polls configured GitHub repositories for open pull requests.
2. On eligible PRs, it posts a welcome comment listing available commands.
3. When an allowed user comments `/che-ai-assistant <command>`, the bot picks it up on the next poll cycle.
4. The bot marks the comment with a :eyes: reaction (to avoid reprocessing) and runs the corresponding handler.
5. Results are posted back to the PR as a comment update.

## Available Commands

| Command | Description |
|---------|-------------|
| `/che-ai-assistant generate-che-doc` | Generate a documentation PR based on the PR's changes |
| `/che-ai-assistant ok-pr-review` | Run a comprehensive PR review (summary, code review, deep review, impact analysis) |
| `/che-ai-assistant help` | Show available commands |

## Prerequisites

- Go 1.25+
- A GitHub token with access to the watched repositories
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) installed (used by handlers that invoke Claude)

## Configuration

All configuration is via environment variables.

### Required

| Variable | Description |
|----------|-------------|
| `CHE_AI_ASSISTANT_GITHUB_TOKEN` | GitHub API token for the bot |
| `CHE_AI_ASSISTANT_WATCH_REPOS` | Comma-separated list of repositories to watch (e.g., `eclipse-che/che-server,eclipse-che/che`) |
| `CHE_AI_ASSISTANT_ALLOWED_USERS` | Comma-separated list of GitHub usernames authorized to trigger commands |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `CHE_AI_ASSISTANT_POLL_INTERVAL` | `5m` | How often to poll for new PR comments |
| `CHE_AI_ASSISTANT_TASK_TIMEOUT` | `30m` | Maximum time a handler can run |
| `CHE_AI_ASSISTANT_MAX_CONCURRENT` | `1` | Maximum number of handlers running concurrently |
| `CHE_AI_ASSISTANT_TEMPLATES_DIR` | `templates` | Directory containing prompt templates |
| `CHE_AI_ASSISTANT_LOG_FILE` | `/tmp/che-ai-assistant.log` | Log file path |
| `CHE_AI_ASSISTANT_DOC_GENERATOR_GITHUB_TOKEN` | | GitHub token used inside DevWorkspaces for doc generation (needs write access to `eclipse-che/che-docs`) |

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

Adding a new subcommand handler requires four steps:

1. Define the subcommand constant
2. Create a prompt template
3. Implement the handler
4. Register the handler

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
    {Type: SubCommandMyCommand, Description: "Short description of what it does"},  // add this
    {Type: SubCommandHelp, Description: "Show this help message"},
}
```

### 2. Create a Prompt Template

Create `templates/my-command.tmpl`. The template is a Go text/template that receives `{{.PRURL}}` — the HTML URL of the triggering pull request.

```
You are an automated assistant. Follow these steps exactly:
1. ...
2. Use information from this PR: {{.PRURL}}
3. ...
```

Requirements:
- The template **must** contain the `{{.PRURL}}` placeholder (enforced at startup).
- The filename (minus `.tmpl`) must match the `SubCommandType` constant value.

### 3. Implement the Handler

Create a new file in `pkg/processor/handlers/`, e.g. `my_command_handler.go`. 
Implement the `Handler` interface.

### 4. Register the Handler

In `pkg/processor/processor.go`, add the handler to the `commandHandlers` map:

```go
var commandHandlers = map[commands.SubCommandType]handlers.Handler{
    commands.SubCommandGenerateCheDoc: handlers.NewGenerateCheDocHandler(),
    commands.SubCommandMyCommand:      handlers.NewMyCommandHandler(),  // add this
}
```

## License

[Eclipse Public License 2.0](https://www.eclipse.org/legal/epl-2.0/)
