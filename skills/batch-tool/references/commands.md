# batch-tool command reference

Checked against batch-tool 1.0.0. If the installed version behaves differently, `batch-tool <command> --help` is authoritative.

Each command below that takes `<repository>...` accepts selectors: `repo`, `project/repo`, `'~label'`, `'~all'`, `'!exclude'`, `'+force'`, or `.`. Every repository that is missing locally is cloned into `<git.directory>/<git.host>/<project>/<repo>` before the operation runs.

Each heading gives the operation class of the per-repository equivalent: read-only, local, shared, or destructive. [safety.md](safety.md) defines the classes and maps every command to the git, PR, or shell operation it runs.

## Contents

- Global flags
- Inspection: `auth status`, `catalog`, `labels`
- Git: `status`, `diff`, `branch`, `commit`, `push`, `update`, `stash`
- Pull requests: `get`, `new`, `edit`, `merge`
- `make`, `exec`
- Exit codes

## Global flags

| Flag | Default | Notes |
| --- | --- | --- |
| `--config <file>` | search path | Use a specific config file instead of searching. A missing or unreadable file exits with code 1. |
| `-o, --style tui\|native` | `tui` | Agents always use `native`. |
| `-p, --print` | false | Print the accumulated output after the run. Also turns off wait-on-exit. |
| `--wait` / `--no-wait` (`-q`) | wait, except on non-TTY stdout | Hold the TUI open after completion. |
| `--sync` | false | One repository at a time. Cannot be combined with `--max-concurrency`. |
| `--max-concurrency N` | CPU count | Parallelism limit. |
| `-e, --env KEY=VALUE` | none | Environment for the subprocesses. Repeatable. |
| `--skip-unwanted` / `--no-skip-unwanted` | from `repos.skip-unwanted` | Filter out repositories that carry `repos.unwanted-labels`. |
| `--sort` / `--no-sort` | from `repos.sort` | Sort the resolved repository list. |
| `-v, --version` | | Print `batch-tool version <v>`. Builds from `go install` print `dev`. |

## Inspection (read-only)

| Command | Flags | Purpose |
| --- | --- | --- |
| `auth status` | `-v, --verbose` adds a truncated SHA-256 digest | Show each project's credential backend, source, and status. Makes no API calls and never prints a token. Exits non-zero if any project fails to resolve. |
| `catalog` | `-f, --flush` refreshes from the SCM API | List cached repository metadata: project, default branch, visibility, and labels. The cache TTL is `repos.cache.ttl` (24h). |
| `labels [selectors...]` (alias `label`) | `-v, --verbose` expands each label | With no arguments, list all labels and aliases. With selectors, print `This matches N repositories, listed below:` and the resolved names. Use this before every command that changes something. |

## Git (`batch-tool git <sub>`)

### `git status <repos>` (read-only)
Runs `git status` in each repository.

### `git diff [--cached] <repos>` (read-only)
Shows unstaged changes. `--cached` shows staged changes instead.

### `git branch -b <name> <repos>` (local; destructive when it resets an existing branch)
Alias: `checkout`. For each repository, the command:
1. Stashes uncommitted changes (scope from `git.stash.scope`).
2. Checks out the default branch and pulls it.
3. Runs `git checkout -B <name>`, which **resets an existing branch with that name**. With `--no-reset` or `git.branch.reset: false`, it runs `git checkout -b` and fails if the branch exists.
4. Restores the stash onto the new branch.

| Flag | Effect |
| --- | --- |
| `-b, --branch` | Branch name. Required. |
| `--reset` / `--no-reset` | Override `git.branch.reset`. |

To branch from a clean worktree, run `git update --discard <repos>` first; that step is the destructive one.

### `git commit {-m <msg> \| --amend [-m <msg>]} <repos>` (local; shared with push; destructive with `--amend` and push)
Refuses to run on the default branch.

| Flag | Effect |
| --- | --- |
| `-m, --message` | Commit message. Required unless `--amend` is set. |
| `-s, --stage all\|tracked\|none` | `all` (default) runs `git add -A`, which includes untracked files. `tracked` is like `git commit -a`. `none` commits only what is already staged. Config key: `git.commit.stage`. |
| `--amend` | Runs `git commit --amend --reset-author`, with `--no-edit` when no `-m` is given. |
| `--push` / `--no-push` | Push after committing. Config key: `git.commit.push`. **`--amend` combined with push implies a force push.** |

### `git push [-f] [--remote <name>] <repos>` (shared; destructive with `-f`)
Runs `git push -u <remote> <current-branch>`. Refuses to push the default branch.

| Flag | Effect |
| --- | --- |
| `-f, --force` | Force push, overwriting remote history. |
| `--remote` | Remote name. Default `origin`, config key `git.remote`. |

### `git update <repos>` (local; destructive with discard)
Checks out the default branch and pulls it.
- **Default (stash mode):** stash local changes, update, then restore the stash.
- **Discard mode** (`--discard` or `git.update.discard: true`): `git reset --hard`, `git clean -fd` (`-x` as well when `git.update.clean-ignored: true`), then submodule deinit and update when `git.update.submodules: true`. **Uncommitted work is destroyed.**

| Flag | Effect |
| --- | --- |
| `--discard` / `--stash` | Choose the mode. Discard mode is destructive. `--stash` overrides a config that turns discard on. |
| `--pull-strategy default\|ff-only\|rebase\|merge` | Maps to plain `git pull`, `--ff-only`, `--rebase`, or `--no-rebase`. Config key: `git.update.pull-strategy`. |

### `git stash <repos>` (read-only)
Runs `git stash list`.

### `git stash push [--scope all|untracked|tracked] <repos>` (local)
Stashes with the message `batch-tool <RFC3339 timestamp>`. Scopes for `--scope`: `all` (default) is `-a` and includes ignored files; `untracked` is `-u`; `tracked` stashes tracked files only. Config key: `git.stash.scope`. A clean worktree is reported as a success.

### `git stash pop [--allow-any] <repos>` (local)
Pops the most recent stash only if batch-tool created it. `--allow-any` lifts that restriction, so the top stash is popped whoever created it. Use it only when the user wants their own manual stashes restored.

## Pull requests (`batch-tool pr <sub>`)

Every `pr` subcommand first checks that a credential resolves for the default project, and fails early if it does not. The repositories must be on a feature branch.

### `pr get <repos>` (read-only)
Alias: `list`. Shows the PR number, title, reviewers, branches, and description for the current branch.

### `pr new <repos>` (shared: equivalent to `gh pr create`)

| Flag | Effect |
| --- | --- |
| `-t, --title` | Defaults to the branch name. |
| `-d, --description` | PR body. |
| `-r, --reviewer` / `-R, --team-reviewer` | Repeatable. When omitted, reviewers come from `repos.reviewers` and `repos.team-reviewers`, keyed by repository or `~label`. |
| `-b, --base-branch` | Defaults to the repository's default branch. |
| `--draft` / `--no-draft` | Open the PR as a draft. |

### `pr edit <repos>` (shared: equivalent to `gh pr edit`)
Updates the PR for the current branch. Takes the same `-t`, `-d`, `-r`, `-R`, and `--draft` flags as `pr new`. Reviewers are **appended** unless `--reset-reviewers` is set, in which case they replace the existing list.

### `pr merge <repos>` (destructive: equivalent to `gh pr merge`, which is irreversible)

| Flag | Effect |
| --- | --- |
| `-m, --method merge\|squash\|rebase` | Default comes from `git.default-merge-method` (`squash`). |
| `--check` / `-f, --force` | `--check` verifies approval and mergeability first (GitHub only; the status can be stale). `-f` skips the check. The check is off by default. |

After merging, run `batch-tool git update <repos>` to return to the default branch.

## `make [-t <target>]... <repos>` (the class of each repository's recipe)
Runs `make <targets...>` in each repository. Targets come from `-t, --target`, which is repeatable; with no `-t`, it runs the default target. Fails if a repository has no Makefile. Classify what the recipe does, not the target's name: the same target can run different commands in different repositories. A recipe you have not read counts as destructive. Use `--sync` for heavy builds.

## `exec` (the class of the command or script it runs)
Alias: `sh`. Runs arbitrary code in each repository, with no sandbox. `exec -c "go test ./..."` is exactly as risky as running `go test ./...` in each repository yourself.

| Flag | Effect |
| --- | --- |
| `-c, --script "<cmd>"` | Runs through `sh -c` in the repository directory. |
| `-f, --file <path>` | Runs an executable file directly. It must have the execute bit set. |
| `-a, --arg <arg>` | Repeatable. Requires `-f`. |
| `-y, --force` | Skip the interactive y/N prompt, which only a person at a terminal can answer. Without `-y`, a non-interactive stdin makes the command fail. **Agents pass it once the command and scope are authorized** under [safety.md](safety.md). |

Exactly one of `-c` or `-f` is required.

## Exit codes

| Code | Meaning | Agent action |
| --- | --- | --- |
| 0 | Every repository succeeded. | Report the results. |
| 1 | Setup, configuration, flag, or argument error. Usage is printed. | Fix the invocation or configuration, and involve the user if it is a setup problem. |
| 2 | One or more repositories failed at runtime. | Report the failures per repository. Retry only the failed repositories, and only after the cause is understood. |
