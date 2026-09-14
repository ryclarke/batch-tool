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

Repository discovery and pull request operations require a credential. Credentials
are resolved **per project**, so each organization can authenticate with a
different account.

The quickest way to get started is to export a token:

- GitHub: create a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- Bitbucket: create an [API token](https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/)

```bash
export AUTH_TOKEN=your-token
```

If you use GitHub and already have the [GitHub CLI](https://cli.github.com)
installed and authenticated, that works with no configuration at all. `gh` is
never required, and is only consulted when `git.provider` is `github`.

#### Credential Backends

Set `auth.provider` to choose how credentials are read. No configuration field
ever holds a credential itself: each one names an environment variable, a
command, or a `gh` account to read it from.

| Backend | Reads the credential from |
| --- | --- |
| `auto` (default) | `AUTH_TOKEN`, then `GH_TOKEN`/`GITHUB_TOKEN`, then the `gh` CLI |
| `env` | the environment variable named by `auth.env` |
| `gh` | `gh auth token`, optionally for the account named by `auth.account` |
| `command` | the standard output of the command in `auth.command` |
| `none` | nothing - requests are made unauthenticated |

#### Multiple Organizations

Add an `auth.owners` entry to give a project its own credential. Each entry
overrides only the fields it sets, falling back to the top-level `auth` settings
for the rest.

```yaml
git:
  provider: github
  project: your-username
  projects:
    - your-work-org

auth:
  provider: auto

  owners:
    your-username:
      provider: gh
      account: your-personal-github-login
    your-work-org:
      provider: gh
      account: your-work-github-login
    partner-org:
      provider: env
      env: PARTNER_ORG_TOKEN
    some-public-org:
      provider: none
```

To use two GitHub accounts this way, log into both and let `auth.account` select
between them. Batch Tool never changes which account is active.

```bash
gh auth login   # repeat for each account
gh auth status  # lists the account names to use for auth.account
```

The `command` backend reads a credential from any external tool, such as a
password manager. The command is given as a list and is executed directly, never
through a shell:

```yaml
auth:
  provider: command
  command: ["op", "read", "op://Private/GitHub/token"]
```

#### Continuous Integration

Inside GitHub Actions the default `auto` backend picks up the standard
`GITHUB_TOKEN` with no configuration:

```yaml
- run: batch-tool pr get '~all'
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Environment variables are always preferred over the `gh` CLI, so CI runs stay
deterministic and never spawn a subprocess.

#### Verifying Your Setup

`batch-tool auth status` reports which credential each project resolves to. It
makes no API requests and never prints a credential. Add `-v` to include a
truncated one-way digest, which is enough to confirm that two projects resolved
to different credentials:

```console
$ batch-tool auth status -v
PROJECT        BACKEND   SOURCE                     STATUS          DIGEST
partner-org    env       $PARTNER_ORG_TOKEN         ok              sha256:1b77ca52
public-org     none      unauthenticated            none required   -
your-username  gh        gh (user: personal-login)  ok              sha256:4f2a1c9b
your-work-org  gh        gh (user: work-login)      ok              sha256:8e0d33a1
```

The command exits non-zero if any project fails to resolve, so it also works as
a CI preflight. Note that a resolved credential is known to exist but is not
known to be valid or to carry the necessary access.

### 3. Try a Safe Read-Only Command

```bash
batch-tool auth status
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

### Project Resolution

Repositories are tracked internally as `project/name`. When you supply an explicit project prefix it is used as given, without consulting the catalog. A bare name is resolved against the catalog, preferring `git.project` and then each entry of `git.projects` in the order you configured them. A name that is unknown to the catalog falls back to `git.project`.

Output trims the `git.project` prefix, so repositories in your default project display as bare names and everything else stays qualified.

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

#### Choosing What Gets Committed

By default `git commit` stages everything, including untracked files. Use `-s`/`--stage` to narrow that:

- `all` (default): stage every change, including untracked files
- `tracked`: stage modifications and deletions to tracked files only, like `git commit -a`
- `none`: commit only what you already staged yourself

```bash
batch-tool git commit -s tracked -m "Tweak config" '~platform'
```

`git commit` does not push. Pass `--push` when you want it to, or set `git.commit.push` to make that the default.

#### Updating and Stashing

`git update` stashes local changes and restores them after updating, so it will not throw away work by default. Pass `--discard` to reset the worktree instead, or set `git.update.discard` to make that the default. `--stash` forces the stash behavior back on when the config opts into discarding. Use `--pull-strategy` to control how diverged history is reconciled (`default`, `ff-only`, `rebase`, or `merge`).

`git stash push` captures untracked and ignored files by default. Use `--scope untracked` to leave ignored files such as build artifacts and `.env` in place, or `--scope tracked` to stash tracked files only.

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

### Git Command Defaults

Each `git` subcommand reads its persistent defaults from a matching config group, so you can set the behavior you want once instead of repeating flags. Every key below defaults to the behavior Batch Tool had before it became configurable.

```yaml
git:
  remote: origin # remote used by 'git push' and 'git commit --push'

  commit:
    stage: all # "all", "tracked", or "none"
    push: false # push automatically after committing

  update:
    discard: false # discard local changes instead of stashing and restoring them
    pull-strategy: default # "default", "ff-only", "rebase", or "merge"
    clean-ignored: false # also delete ignored files when discarding changes
    submodules: true # re-initialize submodules after discarding changes

  stash:
    scope: all # "all", "untracked", or "tracked"

  branch:
    reset: true # reset branches that already exist instead of failing
```

`git.stash.scope` is grouped under `stash` rather than `update` because it applies to every stash Batch Tool takes, including the ones created by `git update` and `git branch`.

`git.stash-updates` has been removed, and `git update` now stashes by default rather than only when asked. If the old key is still present in your config file, `batch-tool` prints a warning and ignores it. Delete `git.stash-updates: true`, since it is now the default behavior; replace `git.stash-updates: false` with `git.update.discard: true` to keep discarding changes. The `--no-stash` flag is likewise gone in favor of `--discard`.

### Aliases and Unwanted Labels

Use `repos.aliases` to define local groupings that behave like labels. Alias members may be given as bare or `project/name` values, in any mix; each is resolved through the catalog when the alias is loaded. Use `repos.unwanted-labels` together with `repos.skip-unwanted` to keep deprecated or experimental repositories out of broad operations unless you explicitly force them in.

### Default Reviewers

Use `repos.reviewers` or `repos.team_reviewers` to preconfigure the reviewers you usually request for a given repository or label. Repository keys may be either bare or `project/name`, and label keys use the `~label` form.

## Troubleshooting

- Authentication errors: run `batch-tool auth status` to see which credential each project resolves to
- Repository not found: confirm the repository name, default project, and cached catalog data
- Unexpected matches: run `batch-tool labels <selectors...>` to inspect how your filters resolve
- Interactive hangs in automation: use `--style native` or `--no-wait`
- Long-running commands: reduce concurrency with `--sync` or `--max-concurrency` limits

For command-specific help, run:

```bash
batch-tool [command] --help
```

If you are contributing to the project itself, see [CONTRIBUTING.md](CONTRIBUTING.md).

## 🤖 AI Use Disclosure

This project uses AI assistance in a supervised, human-in-the-loop workflow.

AI assistance has been used most heavily for:

- documentation drafting and style iteration
- early TUI output scaffolding and presentation experiments
- expanding unit test cases and coverage scenarios

AI is not the source of truth for behavior, correctness, or project structure. The maintainer is responsible for final design choices, code review, and acceptance.

This is not a prompt-driven "vibe-coded" project. Changes are expected to pass normal engineering controls and quality expectations, and any prompt-based contributions are carefully vetted.

Users should expect intentionally reviewed software, not unattended AI output.
