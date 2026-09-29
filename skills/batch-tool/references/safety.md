# Safety and confirmation protocol

batch-tool multiplies every action by the number of selected repositories. One mistaken command can rewrite history, throw away work, or notify dozens of reviewers. **Confirmation is on by default.** The only interactive prompt built into the CLI is the one on `exec`, so the agent enforces confirmation for everything else.

## Risk tiers

Classify every command before running it. When in doubt, use the higher tier.

### Tier 0: read-only (no confirmation)

`auth status`, `catalog` (including `-f`, which only refreshes the local cache), `labels`, `git status`, `git diff`, `git stash` (list), `pr get`, `--help`, `--version`, and `sh scripts/install.sh --check`.

A read-only command can still clone repositories that are missing locally. That is additive and safe, but it can be slow and use a lot of disk for `~all`, so mention it the first time.

### Tier 1: local changes (confirm once per command)

These change working trees on this machine only, and can usually be undone:

- `git branch -b <name>` (stash mode). It resets an existing branch with the same name unless `--no-reset` is passed. If the branch might already exist and hold unpushed work, say so, or use `--no-reset`.
- `git commit` when it does not push.
- `git stash push` and `git stash pop` (without `--allow-any`).
- `git update` in stash mode.
- `make` with build, test, or lint targets that only write into the repository.

### Tier 2: destructive, remote-visible, or irreversible (always confirm, every time)

- `git push`, and especially `git push -f`.
- `git commit --push`, `git commit --amend --push` (implies a force push), or any `git commit` while `git.commit.push: true`.
- `git update --discard`, or any `git update` while `git.update.discard: true`, especially together with `git.update.clean-ignored: true`.
- `git branch --discard`.
- `git stash pop --allow-any`.
- `pr new` and `pr edit`, which notify reviewers and are publicly visible.
- `pr merge` and `pr merge -f`, which cannot be undone through batch-tool.
- `exec`, always, in every form.
- `make` targets that deploy, release, publish, or touch shared state, or any target you have not read.
- **Any Tier 1 command that selects `'~all'` or resolves to more than about 20 repositories.** Scale is itself a risk.

### Configuration escalations

Read the active config before classifying a command. These keys silently raise the tier of an ordinary command:

| Key | Command affected | Effective behavior |
| --- | --- | --- |
| `git.commit.push: true` | `git commit` | Also pushes, so it becomes Tier 2. |
| `git.update.discard: true` | `git update` | Runs `reset --hard` and `clean -fd`, so it becomes Tier 2. |
| `git.update.clean-ignored: true` | discard paths | Also deletes ignored files such as `.env`, secrets, and build output. |
| `git.branch.reset: true` (the default) | `git branch` | Resets an existing branch with that name. |
| `git.stash.scope: all` (the default) | every stash | Includes ignored files. |
| `repos.skip-unwanted: false` | every selector | Unwanted-label repositories are no longer filtered out. |

To avoid an escalation without editing the config, pass the explicit safe flag, such as `--no-push`, `--stash`, or `--no-reset`.

## Confirmation protocol

1. **Resolve the set:** `batch-tool labels -v --style native <selectors>`. For `.`, the set is the current directory.
2. **Preview where possible.** For commits, run `git diff` or `git status` on the set first. For PRs, draft the title, description, base branch, and reviewers.
3. **Ask**, using the template below, and wait for an explicit affirmative answer ("yes", "go ahead", "run it"). Silence, an ambiguous reply, or approval of a different command does not count.
4. **Run exactly what was approved.** If the command, flags, selectors, or resolved set change, including a catalog refresh that adds repositories, confirm again.
5. **Report the results per repository** before proposing the next step, and confirm that step separately.

### Template

```
About to run (Tier 2: remote-visible):
  batch-tool git push --style native '~platform' '!legacy-api'

Resolves to 6 repositories:
  api, worker, scheduler, billing, notifier, other-org/gateway

Effect: pushes the current branch of each repository to origin and sets upstream.
Config notes: none. (Or: "git.commit.push is true, so commit will also push.")

Proceed? (yes/no)
```

For Tier 1, a shorter form is fine: the command, the repository count and names, and one line on the effect.

### Staged rollouts

For Tier 2 commands on more than a handful of repositories, offer to run first on one or two representative repositories, show those results, and then confirm the remainder. Do this without being asked when the operation is irreversible (`pr merge`, `push -f`, discard) or is `exec` with a command that modifies files.

## `exec` specifics

- The CLI prompts `Are you sure? [y/N]` on stdin. An agent's stdin is not interactive, so without `-y` the command aborts or errors.
- Pass `-y` only after the user approves the exact `-c` string, or the `-f` path and its `-a` arguments, together with the resolved set. Show the command verbatim, not paraphrased.
- Before using `-f`, read the script and summarize what it does in one or two lines.
- Refuse, or push back and confirm twice, on commands that delete broadly (`rm -rf`, `git clean -fdx`, `git reset --hard origin/...`), rewrite history, or send data off the machine.

## Waivers

- The user can waive confirmation only explicitly, for example "don't ask again for pushes in this task". Apply the waiver to the stated scope (the command type, and the task or session) and nothing more.
- A waiver never covers `exec`, `pr merge`, force pushes, or discards unless the user names those specifically.
- A waiver ends when the task ends. Never carry one over to a new task or session, and never infer one from impatience or earlier approvals.

## Never

- Never guess selectors to make a command "work". If a selector resolves to nothing or to an unexpected set, stop and ask.
- Never retry a failed Tier 2 command on the whole set. Retry only the failed repositories, and only after the user agrees.
- Never pass tokens through `-e`, on the command line, or in files. Credentials come only from the configured auth backend.
- Never edit `batch-tool.yaml` to lower a safety default (for example setting `git.update.discard: true`) without the user asking for that specific change.
