# Batch Tool

Batch Tool is a command-line utility for running the same workflow across many repositories at once. Make, git, pull request, and shell operations are supported.

It is built for people who routinely work across a repo fleet and want one consistent way to:

- select repositories by name, SCM label, or locally-configured alias
- fan out git operations safely
- open, edit, and merge pull requests across supported providers
- run make targets or custom commands across a coordinated set of repositories

## Why Use It

- One command surface for multi-repository work
- Fast repository selection with aliases, labels, and exclusions
- Interactive TUI output by default, with plain line-by-line output for scripts and CI
- Shared configuration for repository groups, unwanted labels, and reviewers
- Support for GitHub and Bitbucket pull request workflows

## Install

Choose the lowest-friction option that fits your environment.

### Pre-built Binary

Download the archive for your platform from [the latest release](https://github.com/ryclarke/batch-tool/releases/latest), unpack it, and place `batch-tool` somewhere on your `PATH`.

### Go Install

If you already use Go locally, install the latest tagged release with:

```bash
go install github.com/ryclarke/batch-tool@latest
```

This installs the binary into your Go bin directory, typically `$GOPATH/bin` or `~/go/bin`. Make sure that directory is on your `PATH`.

For local builds, release packaging, and contributor setup, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Quick Start

### 1. Create a Config File

Batch Tool looks for `batch-tool.yaml` in:

- the current working directory
- your user config directory
- `$XDG_CONFIG_HOME` when set
- the directory containing the executable

You can also point to a specific file with `--config`.

Start with this minimal example:

```yaml
git:
  provider: github
  project: your-org-or-username

repos:
  unwanted-labels:
    - deprecated
    - poc
  aliases:
    app:
      - web-app
      - mobile-app
    platform:
      - api
      - worker
  reviewers:
    api:
      - backend-team
```

The only field you must set to get started is `git.project`.

### 2. Configure Authentication

Repository discovery and pull request operations require an API token.

- GitHub: create a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- Bitbucket: create an [API token](https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/)

Prefer setting the token through `AUTH_TOKEN` in your environment.

### 3. Try a Safe Read-Only Command

```bash
batch-tool git status repo1 repo2
batch-tool labels '~app'
batch-tool catalog
```

## Repository Selection

Most commands accept one or more repository selectors.

- `repo1` or `project/repo1`: select a repository directly
- `'~backend'`: select all repositories with the SCM label or configured alias
- `'!repo2'` or `'!~deprecated'`: exclude repositories from the working set
- `'+repo3'` or `'+~experimental'`: force inclusion even if the repo would normally be filtered out
- `.`: run the command once against the current working directory only

ℹ️ `~all` is always available and expands to every discovered repository in the configured project.

Examples:

```bash
batch-tool git status '~app' '!mobile-app'
batch-tool git status '+~experimental' '!~deprecated'
batch-tool pr get .
```

Quote selectors that contain `!`, `+`, or `~` so your shell does not expand them first.

## Core Workflows

### Git Operations

```bash
batch-tool git status '~app'
batch-tool git branch -b feature/new-checkout '~app' '~platform'
batch-tool git diff '~platform'
batch-tool git commit -m "Roll out config update" '~platform'
batch-tool git push '~platform'
batch-tool git update '~all'
```

`git update` can optionally stash and restore local changes if `git.stash-updates` is enabled or you pass `--stash`.

### Pull Request Operations

```bash
batch-tool pr new -t "Add checkout flow" -d "Summary of changes" '~app'
batch-tool pr edit -r alice -R my-org/platform-team '~platform'
batch-tool pr merge -m squash --check '~platform'
```

PR commands validate that you are not operating from the repository's base branch.

### Make and Exec

```bash
batch-tool make -t test '~platform'
batch-tool exec -c "go test ./..." '~platform'
batch-tool exec -f ./scripts/deploy.sh -a staging '~app'
```

⚠️ `exec` is intentionally explicit and prompts for confirmation before running unless you pass `-y`. This feature is powerful but __dangerous__, so use it with caution, especially with destructive commands.

## Output Modes

Batch Tool supports two output styles:

- `tui` (default): interactive progress display with scrolling and per-repository output
- `native`: plain line-by-line stdout — each repository's output is printed as it arrives, with no TUI chrome. Reliable in scripts, CI pipelines, and non-interactive terminals.

Use `--style native` when you want straightforward terminal output without the interactive display.

The TUI can be cancelled at any time with `q`, `Esc`, or `Ctrl+C`. Cancellation propagates to in-flight subprocesses, not just the screen.

Useful global flags:

- `--config`: use a specific config file
- `--style` / `-o`: choose `tui` or `native`
- `--print` / `-p`: print accumulated output after the run completes
- `--sync`: run repositories one at a time
- `--max-concurrency`: control parallelism directly
- `--env` / `-e`: inject environment variables into executed commands

## Configuration Notes

### Repository Directory

Repositories are cloned beneath `git.directory` using the provider host, project, and repository name. If you do not set `git.directory`, Batch Tool defaults to `$GOPATH/src` when `GOPATH` is available and otherwise falls back to the current working directory.

### Aliases and Unwanted Labels

Use `repos.aliases` to define local groupings that behave like labels. Use `repos.unwanted-labels` together with `repos.skip-unwanted` to keep deprecated or experimental repositories out of broad operations unless you explicitly force them in.

### Default Reviewers

Use `repos.reviewers` or `repos.team_reviewers` to preconfigure the reviewers you usually request for a given repository or label.

## Troubleshooting

- Authentication errors: verify `AUTH_TOKEN` and your provider configuration
- Repository not found: confirm the repository name, default project, and cached catalog data
- Unexpected matches: run `batch-tool labels <selectors...>` to inspect how your filters resolve
- Interactive hangs in automation: use `--style native` or `--no-wait`
- Long-running commands: reduce concurrency with `--sync` or `--max-concurrency` limits

For command-specific help, run:

```bash
batch-tool [command] --help
```

If you are contributing to the project itself, see [CONTRIBUTING.md](CONTRIBUTING.md).
