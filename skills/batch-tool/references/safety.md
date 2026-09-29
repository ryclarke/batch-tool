# Safety and authorization

A batch-tool command is the same operation you already know from git, a PR tool, `make`, or a shell, repeated in every selected repository. Judge it that way:

**effective risk = the per-repository operation × the set of repositories**

Nothing about batch-tool makes an operation more or less dangerous than running it by hand in each repository. It only multiplies the effect. So:

1. **Translate** the batch-tool command into the git, PR, or shell operations it runs in each repository.
2. **Authorize the operation** exactly as you would for one repository: use the permissions and instructions you already have for that git, PR, or shell operation.
3. **Authorize the scope** separately, because permission to do something is not permission to do it everywhere.

Destructive operations still require confirmation whenever they span several repositories, even when the operation itself was requested. This is the default.

## Step 1: Translate

Operation classes:

- **read-only**: changes nothing.
- **local**: changes this machine's working trees, and git can undo it.
- **shared**: visible to other people but reversible, for example by deleting a branch or closing a PR.
- **destructive**: loses work, rewrites shared history, or cannot be undone.

| batch-tool command | Runs in each repository | Class |
| --- | --- | --- |
| `git status`, `git diff [--cached]`, `git stash` | the same git command (`stash list` for bare `stash`) | read-only |
| `labels`, `catalog`, `auth status` | local lookups; `catalog -f` reads repository metadata from the SCM API | read-only |
| `pr get` | `gh pr view` for the current branch | read-only |
| `git branch -b X` | `git stash push`, `git checkout <default>`, `git pull`, `git checkout -B X`, `git stash pop` | local; **destructive if branch `X` already exists with commits found nowhere else**, since `-B` resets it. Pass `--no-reset` to use `-b`, which fails instead. |
| `git update --discard` then `git branch -b X` | a clean start: the discard path below, then the branch steps above | destructive (from the discard step) |
| `git commit -m M` | `git add -A` (or `-a` with `-s tracked`, nothing with `-s none`), then `git commit -m M`; refuses the default branch | local |
| `git commit --amend` | `git commit --amend --reset-author` | local; rewrites the last commit |
| `git commit ... --push` | the commit, then `git push -u <remote> <branch>` | shared |
| `git commit --amend --push` | the amend, then `git push -f` | destructive |
| `git push` | `git push -u <remote> <branch>`; refuses the default branch | shared |
| `git push -f` | `git push -u -f <remote> <branch>` | destructive |
| `git update` | `git stash push`, `git checkout <default>`, `git pull [--ff-only\|--rebase\|--no-rebase]`, `git stash pop` | local |
| `git update --discard` | `git reset --hard`, `git clean -fd` (`-fdx` with `clean-ignored`), submodule deinit/update, then checkout and pull | destructive |
| `git stash push` / `pop` | `git stash push [-a\|-u] -m "batch-tool <time>"` / `git stash pop`, which pops only batch-tool stashes | local |
| `git stash pop --allow-any` | `git stash pop` for whatever stash is on top | local, but may apply someone else's stash |
| `pr new` | `gh pr create`, or the Bitbucket equivalent, **plus reviewer requests from `repos.reviewers` and `repos.team-reviewers`** when no `-r` or `-R` is given | shared |
| `pr edit` | `gh pr edit`; `--reset-reviewers` replaces the reviewer list | shared |
| `pr merge` | `gh pr merge` (squash by default), which changes the default branch | destructive |
| `make -t T` | `make T` with **that repository's** recipe | the class of the recipe (see "exec and make") |
| `exec -c C` | `sh -c C` in the repository directory | the class of `C` |
| `exec -f F -a A...` | `F A...` in the repository directory | the class of the script `F` |

The guards batch-tool adds (refusing to commit to or push the default branch, stashing by default, popping only its own stashes) only ever lower the risk compared with the raw commands. Never rely on them to justify skipping a step.

### Configuration changes the translation

Read the active config before translating. These keys change what a command runs:

| Key | Changes |
| --- | --- |
| `git.commit.push: true` | `git commit` also pushes, so it becomes shared, or destructive with `--amend`. Pass `--no-push` to prevent it. |
| `git.update.discard: true` | `git update` runs the discard path, so it becomes destructive. Pass `--stash` to prevent it. |
| `git.update.clean-ignored: true` | Discarding also deletes ignored files such as `.env` and build output. |
| `git.branch.reset: false` | `git branch` uses `-b` and fails rather than resetting. |
| `git.stash.scope` | What stashes capture; `all` (the default) includes ignored files. |
| `repos.skip-unwanted: false` | Selectors stop filtering out unwanted-label repositories, which widens the scope. |
| `repos.reviewers`, `repos.team-reviewers` | Who `pr new` notifies when you do not pass reviewers. |

## Step 2: Authorize the operation

Use the same sources of authorization you would use for the single-repository operation. In priority order:

1. **The user's instructions in this conversation**, such as "open PRs for this", "push the branches", or "run the tests".
2. **Standing policy**: the host tool's permission settings and allowlists, project rules such as `AGENTS.md`, and the user's configured preferences.
3. **This skill's defaults**, which apply when neither of the above says anything:
   - read-only: run it.
   - local: run it when it is a step of the task the user asked for.
   - shared: ask first.
   - destructive: always ask.

If you would run `git push` or `gh pr create` in one repository without asking, `git push` and `pr new` are the same operations. If you would ask before running them in one repository, ask here too. An authorization for a single-repository operation covers only that operation. For example, permission to push does not cover force pushing, and permission to open PRs does not cover merging them.

For shared operations, include in the request or report anything the raw command would not have done on its own. For `pr new`, that means the reviewers who will be notified from config.

## Step 3: Authorize the scope

The operation's authorization says *what* may be done. The scope says *where*. Resolve it before every write:

```bash
batch-tool labels -v --style native <selectors>
```

The scope counts as authorized when the user supplied the selectors or the repository list, or described a set that the resolved list clearly matches. Show the resolved list before the first write in any case.

Confirm the scope before writing, even for an authorized operation, when:

- you chose or translated the selector yourself, for example when the user said "the backend services" and you picked `'~backend'`.
- the resolved set includes repositories the user did not mention: a broader label than expected, `'~all'`, force-included (`+`) repositories, or unwanted-label repositories because `--no-skip-unwanted` is in effect.
- the set changed after approval, for example because a catalog refresh added repositories.
- a selector resolved to nothing. Stop and ask; never widen it to make the command work.

## When to stop and confirm

Confirm before running, and wait for an explicit yes, when **any** of these holds:

- The per-repository operation is **destructive** and runs in more than one repository. This holds even if the user asked for the operation, because it checks the resolved set immediately before an irreversible step. Only an explicit waiver removes it (see "Waivers").
- You have no authorization for the per-repository operation (Step 2).
- The scope needs confirming (Step 3).
- You cannot classify the payload of an `exec` or `make` command (see "exec and make").

Otherwise, run it, and report the resolved set and the results.

## exec and make

`exec` and `make` are as risky as what they run, no more and no less. Classify the payload, then apply Steps 2 and 3:

- `exec -c "go test ./..."` is `go test` in each repository: local, and authorized wherever running the tests is.
- `exec -c "git log -1 --format=%H"` is read-only.
- `exec -c "sed -i 's/v1/v2/' go.mod"` is a local edit that git can revert.
- `exec -c "git push origin --delete old-branch"`, `rm -rf`, `git clean -fdx`, `git reset --hard origin/...`, piping downloads into a shell, and uploading data off the machine are destructive, or need approval that is explicit and specific to that command.

Rules for payloads:

- **Read before classifying.** For `-f`, read the script. For `make`, read the target's recipe **in each repository**: the same target name can do different things in different repositories. Read a representative sample, and search the others for the recipe's commands if the set is large.
- **If the payload is opaque, it is destructive.** That covers binaries, scripts fetched at run time, recipes that call other tools you cannot see into, and anything you do not understand. Treat it as destructive until the user confirms what it does.
- **Pass the command exactly.** Run exactly the `-c` string, or the `-f` path and `-a` arguments, that was authorized. If you change it, re-authorize.
- **`-y` is plumbing, not a risk signal.** The CLI's `Are you sure? [y/N]` prompt is meant for a person at a terminal, and an agent cannot answer it. Pass `-y` once the payload and scope are authorized under the rules above. Never pass it to get around them.

## Confirmation protocol

1. Resolve the set (Step 3). For `.`, the set is the current directory.
2. Preview where you can. For commits, run `git diff` or `git status` on the set first. For PRs, draft the title, description, base branch, and reviewers, including the ones that come from config.
3. Ask using the template below, and wait for an explicit affirmative answer. Silence, an ambiguous reply, or approval of a different command does not count.
4. Run exactly what was approved. If the command, flags, selectors, or resolved set change, confirm again.
5. Report the results per repository before starting the next step. The next step goes through Steps 1 to 3 again.

```
About to run: batch-tool git update --discard --style native '~platform'
In each repository: git reset --hard; git clean -fd; checkout main; git pull
  (destructive: uncommitted changes are lost)
Resolves to 6 repositories: api, worker, scheduler, billing, notifier, other-org/gateway
Config notes: git.update.clean-ignored is true, so ignored files (.env, build output) are deleted too.
Proceed? (yes/no)
```

Lead with the per-repository equivalent. The user already knows what the underlying git or shell operation means.

### Staged rollouts

For a destructive operation on more than a handful of repositories, offer to run it on one or two representative repositories first, show those results, and then confirm the rest. Do this without being asked for `pr merge`, force pushes, discards, and `exec` or `make` payloads that modify files outside git's reach.

## Waivers

- The user can waive confirmation only explicitly, and only for a stated operation and scope, for example "you don't need to ask before pushing in this task". Apply the waiver to exactly that and nothing more.
- A waiver for one operation never covers a destructive neighbor of it. Waiving confirmation for pushes does not cover force pushes; waiving it for PR creation does not cover merges.
- A waiver ends when the task ends. Never carry one over to a new task, and never infer one from impatience or earlier approvals.

## Never

- Never guess or widen selectors to make a command work.
- Never retry a failed destructive or shared command on the whole set. Retry only the failed repositories, and only after the cause is understood.
- Never pass tokens through `-e`, on the command line, or in files. Credentials come only from the configured auth backend.
- Never edit `batch-tool.yaml` to change what a command does (for example setting `git.update.discard: true` or adding reviewers) unless the user asks for that specific change.
