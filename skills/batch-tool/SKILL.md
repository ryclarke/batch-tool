---
name: batch-tool
description: Installs, updates, configures, and operates batch-tool for running Git, pull-request, Make, and shell workflows across multiple repositories. Use when the user mentions batch-tool or batch-tool.yaml, uses repository selectors, or requests bulk repository operations such as branching, status, commits, pushes, updates, pull requests, Make targets, or shell commands.
compatibility: Requires git with SSH access to the SCM host and network access to GitHub or Bitbucket. The bundled install script needs a POSIX shell, curl or wget, tar with xz support, and sha256sum or shasum. Go and the gh CLI are optional.
metadata:
  author: ryclarke
  version: "1.1.1"
---

# batch-tool

`batch-tool` fans one command out across many git repositories. It resolves repository selectors, clones any missing repositories over SSH, runs the operation in each repository concurrently, and reports results per repository.

Follow this skill in order: preflight, resolve the repository set, translate the command into what it runs in each repository, authorize that operation and the scope, run, and report.

## 1. Preflight

Run these before the first batch-tool command in a session:

```bash
sh scripts/install.sh --check       # prints up-to-date, outdated <cur> -> <latest>, or not-installed
batch-tool auth status              # credential source per project; never prints a token
```

- If the result is `not-installed` or `outdated`, tell the user what you found and install or update with `sh scripts/install.sh` (details below). Installing into a user-owned directory is safe to do once the user agrees to have the tool installed.
- If there is no config file or `auth status` fails, stop and walk the user through [references/setup.md](references/setup.md). Do not guess the provider, project, or credentials.
- Read the active config file. batch-tool prints `Using config file: <path>` on stderr. Note any keys that change what an ordinary command runs in each repository:
  - `git.commit.push: true`: `git commit` also pushes.
  - `git.update.discard: true`: `git update` runs `git reset --hard` and `git clean -fd`.
  - `git.update.clean-ignored: true`: discarding also deletes ignored files such as `.env` and build output.
  - `git.branch.reset: true` (the default): `git branch -b` resets an existing branch with that name.

## 2. Running commands as an agent

- **Always pass `--style native`** (`-o native`). batch-tool already picks native output when stdout is not a terminal, but some agent harnesses run commands in a pseudo-terminal. There, the default TUI takes over the screen and waits for a keypress.
- **Read native output by stream.** Stdout has a `------ <repo> ------` header per repository, followed by that repository's output, in repository order. Stderr has the command line, `ERROR: <repo>: <message>` lines, and a final summary: `N repositories (M failed) | Elapsed: …`, then `Failed: <repos>`. Capture both streams.
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

## 4. Translate, then authorize

A batch-tool command is an ordinary operation repeated in every selected repository: **effective risk = the per-repository operation × the set of repositories**. Read [references/safety.md](references/safety.md) before running anything that writes; it defines the operation classes, configuration effects, and confirmation protocol.

1. **Translate** the command into what it runs in each repository. `pr new` is `gh pr create` (plus reviewers from config). `git push` is `git push -u origin <branch>`. `git update --discard` is `git reset --hard` plus `git clean -fd`. `exec -c C` is `C`. `make -t T` is that repository's recipe for `T`.
2. **Authorize the operation** the way you would for a single repository: the user's instructions first, then the host's permissions and project rules. If you may open a PR or push a feature branch in one repository without asking, `pr new` or `git push` is the same operation. If you would ask first in one repository, ask here too.
3. **Authorize the scope** separately. Show the resolved repository list before the first write. Confirm it first if you chose the selector yourself, or if it includes repositories the user did not mention (`'~all'`, a broader label than expected, force-included repositories).

`exec` and `make` take the class of whatever they run. Destructive operations across several repositories always need an explicit yes by default, even when the user requested the operation. Approval covers only that command and resolved set.

## 5. Common recipes

Commands below omit `--style native` for readability. Always add it.

**Survey state (read-only):**

```bash
batch-tool git status '~app'
batch-tool git diff '~app'
batch-tool pr get '~app'          # repositories must be on a feature branch
```

**Rolling out a change** (authorize each step as its git or PR equivalent):

```bash
batch-tool git branch -b feature/bump-deps --no-reset '~platform'  # stash, update default branch, create branch, restore
# ...edit files in each repo, or use exec classified by the command it runs...
batch-tool git diff '~platform'                          # show the user what will be committed
batch-tool git commit -m "Bump dependencies" '~platform' # stages everything by default; -s tracked or -s none to narrow
batch-tool git push '~platform'
batch-tool pr new -t "Bump dependencies" -d "Why and what" '~platform'
```

For `pr new`, make sure the title, description, base branch, and reviewers are what the user authorized, and show them in your report. Reviewers come from `-r`/`-R`, or from `repos.reviewers` and `repos.team-reviewers` in the config when no flag is given. Use `--draft` if the user wants to review the PRs before others are notified.

**Arbitrary commands:** `batch-tool exec -c "go test ./..." -y '~backend'`, or `-f ./script.sh -a arg1 -a arg2`. `-f` runs that one local file in every repository. A script that lives inside each repository goes through `-c './script.sh'`, and is that repository's own code. Classify the command or script as if you were running it yourself in each repository: `go test` is local, `rm -rf build/` is destructive, and a script you have not read is destructive until you have read it. The `-y` flag only skips a terminal prompt that an agent cannot answer. Pass it once the command and scope are authorized.

For complex, multi-step, or quoting-sensitive operations that should run consistently across repositories, prefer a reviewed local executable with `exec -f` over a long inline `exec -c` command. Show or read the exact script and arguments before authorization. Use `-c` for simple commands or scripts owned by each repository.

For every command and flag, see [references/commands.md](references/commands.md).

## 6. When to stop and involve the user

Stop, explain what is needed, and let the user act whenever the task requires:

- **Creating or rotating credentials,** such as a GitHub personal access token, a Bitbucket API token, or `gh auth login`. Never ask the user to paste a token into the conversation, never echo one, and never write one to a file.
- **Choosing configuration values:** `git.provider`, `git.host`, `git.project`, `git.projects`, `git.directory`, aliases, and reviewers.
- **SSH access for cloning:** batch-tool clones missing repositories from `ssh://git@<host>/<project>/<repo>.git`.
- **Editing shell startup files** to change `PATH` or export variables. Propose the line and ask before touching the file.
- **Reviewing content others will see** when the user has not already specified or approved it: PR titles, descriptions, reviewers, and commit messages on shared branches.
- **Resolving failures that need judgment:** merge conflicts, stash pop conflicts, failed checks, or protected branches.

Walkthroughs for each case are in [references/setup.md](references/setup.md).

## 7. Reporting results

After each run, summarize the outcome per repository: which succeeded, which failed with the key error line, and what is left to do. Where it applies, suggest the next command in the workflow (for example `git push` after `git commit`), and authorize it like any other step before running it.
