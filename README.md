# che-ai-assistant

A GitHub bot that monitors pull requests and issues, executing AI-powered tasks triggered by comments.

## How It Works

1. The bot polls configured GitHub repositories for opened pull requests and issues (issues must have the `che-ai-assistant` label).
2. It posts a welcome comment listing available commands.
3. When a user comments `/che-ai-assistant <command>`, the bot picks it up on the next poll cycle.
4. The bot marks the comment with a :eyes: reaction and runs the task.
5. Results are posted back to the pull request or issue.

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
| `/che-ai-assistant help` | Show available commands |

## Development

### Build and Run

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Clean
make clean
```

### Configuration

All configuration is via environment variables.

| Variable | Default | Description                                                                                    |
|----------|---------|------------------------------------------------------------------------------------------------|
| `CHE_AI_ASSISTANT_GITHUB_TOKEN` | *required* | GitHub API token                                                                               |
| `CHE_AI_ASSISTANT_GITHUB_REPOSITORIES` | *required* | Comma-separated list of repositories to watch (e.g., `eclipse-che/che-server,eclipse-che/che`) |
| `CHE_AI_ASSISTANT_GITHUB_USERS` | *required* | Comma-separated list of GitHub usernames authorized to trigger commands                        |
| `CHE_AI_ASSISTANT_SKILLS_REPOSITORY` | *required* | Repository containing skills for tasks                                                         |
| `CHE_AI_ASSISTANT_MCP_SERVER_URL` | *required* | MCP server URL                                                                                 |
| `CHE_AI_ASSISTANT_GITHUB_POLL_INTERVAL` | `5m` | How often to poll for new comments                                                             |
| `CHE_AI_ASSISTANT_TASK_TIMEOUT` | `30m` | Maximum time a task can run                                                                    |
| `CHE_AI_ASSISTANT_MAX_CONCURRENT_TASKS` | `1` | Maximum number of tasks running concurrently                                                   |
| `CHE_AI_ASSISTANT_PROMPTS_DIR` | `./prompts` | Directory containing prompt templates                                                          |
| `CHE_AI_ASSISTANT_LOG_FILE` | `./che-ai-assistant.log` | Log file path                                                                                  |
| `CHE_AI_ASSISTANT_STATE_FILE` | `~/state.json` | Persistent state file path                                                                     |
| `CHE_AI_ASSISTANT_WARN_DIRS_COMMITS` | `.claude,.vscode` | Comma-separated directories to warn about in PR commits                                        |

## Deployment

The `deploy/` directory contains Kubernetes manifests for deploying the bot as a DevWorkspace in Eclipse Che:

- `che-ai-assistant-config.ConfigMap.yaml` — environment variable configuration
- `che-ai-assistant-github-token.Secret.yaml` — GitHub token secret with access to the watched repositories
- `che-ai-assistant-tasks-config.ConfigMap.yaml` — task-specific configuration

### Steps

1. Deploy the [MCP server](https://github.com/che-incubator/che-mcp-server).
2. Configure [Git credentials](https://eclipse.dev/che/docs/stable/end-user-guide/mounting-git-configuration/)).
3. Update the manifests in `deploy/` with your configuration and apply
4. Start a workspace from this repository

## License

[Eclipse Public License 2.0](https://www.eclipse.org/legal/epl-2.0/)
