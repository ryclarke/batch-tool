---
name: batch-tool
description: Installs, updates, configures, and operates batch-tool, a CLI that runs git, pull request, make, and shell workflows across many repositories at once. Use when the user wants to act on several repositories together (bulk branch, commit, push, update, stash, status, or diff), open, edit, or merge pull requests across repos, select repos by name, ~label, alias, !exclude, or +force selectors, run make targets or shell commands across a repo fleet, or mentions batch-tool or batch-tool.yaml. Destructive and remote-visible multi-repo operations require explicit user confirmation.
compatibility: Requires git with SSH access to the SCM host and network access to GitHub or Bitbucket. The bundled install script needs a POSIX shell, curl or wget, tar with xz support, and sha256sum or shasum. Go and the gh CLI are optional.
metadata:
  author: ryclarke
  version: "1.0.0"
---

# batch-tool

`batch-tool` fans one command out across many git repositories. It resolves repository selectors, clones any missing repositories over SSH, runs the operation in each repository concurrently, and reports results per repository.

Follow this skill in order: preflight, resolve the repository set, classify the risk, confirm when required, run, and report.

## 1. Preflight

Run these before the first batch-tool command in a session:

```bash
sh scripts/install.sh --check       # prints up-to-date, outdated <cur> -> <latest>, or not-installed
batch-tool auth status              # credential source per project; never prints a token
```

- If the result is `not-installed` or `outdated`, tell the user what you found and install or update with `sh scripts/install.sh` (details below). Installing into a user-owned directory is safe to do once the user agrees to have the tool installed.
- If there is no config file or `auth status` fails, stop and walk the user through [references/setup.md](references/setup.md). Do not guess the provider, project, or credentials.
- Read the active config file. batch-tool prints `Using config file: <path>` on stderr. Note any keys that make ordinary commands more destructive, because they change the risk tier of a command:
  - `git.commit.push: true`: `git commit` also pushes.
  - `git.update.discard: true`: `git update` runs `git reset --hard` and `git clean -fd`.
  - `git.update.clean-ignored: true`: discarding also deletes ignored files such as `.env` and build output.
  - `git.branch.reset: true` (the default): `git branch -b` resets an existing branch with that name.

## 2. Running commands as an agent

- **Always pass `--style native`** (`-o native`). The default TUI is interactive and meant for people. Non-TTY stdout already disables the wait-on-exit prompt, and `--no-wait` makes that explicit.
- **Quote every selector** that contains `~`, `!`, or `+`, for example `'~backend' '!legacy-api'`.
- **Exit codes:** `0` means success. `1` means a setup, flag, or argument error; read the message and fix the invocation. `2` means one or more repositories failed at runtime. When you get `2`, report which repositories failed and why, and do not rerun the whole set blindly.
- **Concurrency:** it defaults to the CPU count. Use `--sync` for builds or anything with rate limits or shared resources, or `--max-concurrency N`. The two flags cannot be combined.
- **Environment for subprocesses:** `-e KEY=VALUE` (repeatable). Never pass secrets this way on the command line.
- **Discover flags** with `batch-tool <command> --help` whenever this skill and the installed version disagree. The installed binary is the source of truth.

## 3. Resolve the repository set first

Before any command that is not read-only, resolve the selectors and show the result:

```bash
batch-tool labels -v --style native '~backend' '!~deprecated'
```

The output reports `This matches N repositories, listed below:` followed by the names. Selector rules:

| Selector | Meaning |
| --- | --- |
| `repo` or `project/repo` | A single repository. Bare names resolve against `git.project`, then `git.projects` in order. |
| `'~label'` | Every repository with that SCM label (GitHub topic or Bitbucket label) or local alias. |
| `'~all'` | Every repository in the catalog. |
| `'!repo'` or `'!~label'` | Exclude from the set. |
| `'+repo'` or `'+~label'` | Force-include, even if unwanted labels or archived status would filter it out. |
| `.` | Only the current working directory. |

If a match looks wrong or stale, refresh the catalog with `batch-tool catalog -f --style native` and resolve again. If the set is empty, stop and ask; do not widen the selector on your own.

## 4. Classify risk and confirm

Every command falls into one of three tiers. The full rules, the configuration escalations, and the confirmation template are in [references/safety.md](references/safety.md). Read that file before running anything above the read-only tier.

- **Read-only (run freely):** `git status`, `git diff`, `git stash` (list), `labels`, `catalog`, `auth status`, `pr get`, `--help`, `--version`.
- **Local changes (confirm once per command):** `git branch`, `git commit` without push, `git stash push`/`pop`, `git update` in stash mode, and `make` with targets that write files.
- **Destructive, remote-visible, or irreversible (always confirm):** `git push` (especially `-f`), `git commit --push` or `--amend --push`, anything with `--discard`, `pr new`, `pr edit`, `pr merge` (especially `-f`), every `exec`, and any non-read-only command that selects `~all` or resolves to more than about 20 repositories.

Confirmation is on by default. Before running a command that needs it, show the user the exact command, the resolved repositories with a count, and any configuration that changes its effect, then wait for an explicit yes. Approval covers only that command and that repository set. If either changes, confirm again.

## 5. Common recipes

Commands below omit `--style native` for readability. Always add it.

**Survey state (read-only):**

```bash
batch-tool git status '~app'
batch-tool git diff '~app'
batch-tool pr get '~app'          # repositories must be on a feature branch
```

**Rolling out a change** (confirm each state-changing step):

```bash
batch-tool git branch -b feature/bump-deps '~platform'   # stashes, updates default branch, creates branch, restores
# ...edit files in each repo (or use exec, confirmed)...
batch-tool git diff '~platform'                          # show the user what will be committed
batch-tool git commit -m "Bump dependencies" '~platform' # stages everything by default; -s tracked or -s none to narrow
batch-tool git push '~platform'
batch-tool pr new -t "Bump dependencies" -d "Why and what" '~platform'
```

For `pr new`, show the user the title, description, base branch, and reviewers before running. Reviewers come from `-r`/`-R`, or from `repos.reviewers` and `repos.team-reviewers` in the config when no flag is given. Use `--draft` if the user wants to review the PRs before others are notified.

**Editing PRs:** `batch-tool pr edit -t "New title" -r alice '~platform'`. Reviewers are added to the existing list; `--reset-reviewers` replaces it.

**Merging and cleaning up:**

```bash
batch-tool pr merge -m squash '~platform'   # add --check to verify mergeability first (GitHub only)
batch-tool git update '~platform'           # back to the default branch, pulled, with local changes stashed and restored
```

**Stashing around other work:** `git stash push '~x'`, do the work, then `git stash pop '~x'`. By default `pop` refuses to restore stashes that batch-tool did not create.

**Make targets:** `batch-tool make -t test --sync '~backend'`. Multiple `-t` flags run in a single `make` invocation.

**Arbitrary commands:** `batch-tool exec -c "go test ./..." -y '~backend'`, or `-f ./script.sh -a arg1 -a arg2`. The `-y` flag is required because an agent cannot answer the interactive prompt. Pass it only after the user has approved that exact command and repository set in the conversation.

For every command and flag, see [references/commands.md](references/commands.md).

## 6. When to stop and involve the user

Stop, explain what is needed, and let the user act whenever the task requires:

- **Creating or rotating credentials,** such as a GitHub personal access token, a Bitbucket API token, or `gh auth login`. Never ask the user to paste a token into the conversation, never echo one, and never write one to a file.
- **Choosing configuration values:** `git.provider`, `git.host`, `git.project`, `git.projects`, `git.directory`, aliases, and reviewers.
- **SSH access for cloning:** batch-tool clones missing repositories from `ssh://git@<host>/<project>/<repo>.git`.
- **Editing shell startup files** to change `PATH` or export variables. Propose the line and ask before touching the file.
- **Reviewing content others will see,** such as PR titles, descriptions, reviewers, and commit messages on shared branches.
- **Resolving failures that need judgment:** merge conflicts, stash pop conflicts, failed checks, or protected branches.

Walkthroughs for each case are in [references/setup.md](references/setup.md).

## 7. Reporting results

After each run, summarize the outcome per repository: which succeeded, which failed with the key error line, and what is left to do. Where it applies, suggest the next command in the workflow (for example `git push` after `git commit`), and confirm it before running.

## Install and update

```bash
sh scripts/install.sh                 # latest release into $BATCH_TOOL_INSTALL_DIR or ~/.local/bin
sh scripts/install.sh --version v1.0.0
sh scripts/install.sh --dir "$HOME/bin"
sh scripts/install.sh --check         # report only; changes nothing
```

The script downloads the release archive for the current OS and architecture, verifies its SHA-256 checksum against the release checksum file, and installs the binary. If a prebuilt archive is not usable and Go is available, it falls back to `go install github.com/ryclarke/batch-tool@<tag>`. It never uses `sudo` and never edits shell startup files. If the install directory is not on `PATH`, it prints the line the user should add.

On Windows (without WSL), download `batch-tool_windows_amd64.zip` from the [latest release](https://github.com/ryclarke/batch-tool/releases/latest), check it against the release's `checksums.txt`, and put `batch-tool.exe` on `PATH`. If Go is installed, `go install github.com/ryclarke/batch-tool@latest` also works.
