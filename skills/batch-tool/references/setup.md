# Setup and human intervention

Use this file when batch-tool is not installed, has no config file, cannot resolve credentials, or cannot clone repositories. Parts of setup need decisions or secrets that only the user can provide. For each step below, it says whether **the agent** or **the user** does it.

## Setup checklist

```
- [ ] 1. Binary installed and on PATH          (agent, with the user's consent)
- [ ] 2. Config file exists                     (agent drafts, user supplies values and approves the write)
- [ ] 3. Credentials resolve                    (user)
- [ ] 4. SSH clone access works                 (user)
- [ ] 5. Read-only smoke test passes            (agent)
```

## 1. Install

Run `sh scripts/install.sh --check`, then `sh scripts/install.sh` once the user agrees. If the script reports that the install directory is not on `PATH`, show the user the line it prints (for example `export PATH="$HOME/.local/bin:$PATH"`) and ask before adding it to `~/.bashrc`, `~/.zshrc`, or `~/.profile`. Never edit shell startup files without approval.

## 2. Config file

batch-tool reads the first `batch-tool.yaml` it finds in:

1. the OS user config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS)
2. `$XDG_CONFIG_HOME`, or `~/.config` when that is unset
3. the directory containing the executable

The current directory is not searched. `--config <path>` loads any other file instead, and fails with exit code 1 if the file cannot be read.

**A config file can run commands and redirect credentials.** `auth.command` runs an arbitrary program, and `git.host` controls which server receives the token. Pass `--config` only for a file the user wrote or explicitly chose. If the file comes from a cloned repository or anywhere else you did not create it, read it, point out any `auth.*`, `git.host`, `git.provider`, or `git.user` settings, and get the user's confirmation before using it. When a file is loaded, batch-tool prints `Using config file: <path>` on stderr. `--help` and `--version` exit before any config is loaded, so they print nothing about it.

**Ask the user for these values; do not guess them:**

| Key | Question to ask |
| --- | --- |
| `git.provider` | GitHub or Bitbucket? (`github` or `bitbucket`) |
| `git.host` | Needed only for GitHub Enterprise or self-hosted Bitbucket. Defaults to `github.com`. |
| `git.project` | Which user, organization, or workspace holds the repositories? **Required.** |
| `git.projects` | Any additional organizations to include? |
| `git.directory` | Where should repositories be cloned? Defaults to `$GOPATH/src`, or the current directory when there is no GOPATH. Existing checkouts are reused only if they are at `<dir>/<host>/<project>/<repo>`. |

Draft the file, show it to the user, and write it only with their approval. Suggest `~/.config/batch-tool.yaml` for a personal setup.

```yaml
git:
  provider: github            # or bitbucket
  project: your-org           # required
  # host: github.example.com  # only for GitHub Enterprise or self-hosted Bitbucket
  # projects: [other-org]
  # directory: ~/src

repos:
  unwanted-labels: [deprecated, archived-soon]
  aliases:
    platform: [api, worker]
  reviewers:
    "~platform": [alice]
```

Keep the safety defaults: `git.commit.push: false`, `git.update.discard: false`, and `git.update.clean-ignored: false`. Change them only if the user asks for that specific change.

## 3. Credentials (the user does this)

Catalog discovery and every `pr` command need a credential for each project. **No config key ever holds a token.** Config keys name where a token comes from: an environment variable, a command, or a `gh` account.

**Rules for the agent:**
- Never ask for a token in chat, never print or echo one, and never write one to any file, including `batch-tool.yaml`, `.env`, and shell startup files.
- Never run `gh auth token` or read a token variable to "check" it. Use `batch-tool auth status`, which reports only the source and a truncated digest.
- Explain the options below, let the user complete them in their own terminal, then rerun `batch-tool auth status`.

**Options, easiest first:**

1. **GitHub CLI** (GitHub only, no config needed). The user runs `gh auth login` interactively. The default `auto` backend picks it up.
2. **Environment variable.** The user creates a token and exports it in their own shell or secret manager as `AUTH_TOKEN`. On GitHub, `GH_TOKEN` or `GITHUB_TOKEN` also work.
   - GitHub: [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens). A fine-grained token needs Metadata (read), Contents (read and write), and Pull requests (read and write) on the target repositories. A classic token needs the `repo` scope.
   - Bitbucket: [API token](https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/) with repository read and pull request write access.
3. **Password manager command.** For example:

   ```yaml
   auth:
     provider: command
     command: ["op", "read", "op://Private/GitHub/token"]
   ```

   The command runs directly, not through a shell.
4. **Several organizations or accounts:** use a per-project `auth.owners` entry:

   ```yaml
   auth:
     owners:
       your-work-org:
         provider: gh
         account: work-login          # from `gh auth status`
       partner-org:
         provider: env
         env: PARTNER_ORG_TOKEN       # the variable's NAME, not its value
   ```

The `auto` backend (the default) tries `$AUTH_TOKEN` (or whatever `auth.env` names), then the deprecated `auth-token` key, then `GH_TOKEN` or `GITHUB_TOKEN`, then `gh`. If the config still contains a literal `auth-token:` value, tell the user it is deprecated and stores a secret in plain text, and recommend moving it into an environment variable or password manager.

**Verify:** `batch-tool auth status` exits non-zero if any project fails to resolve. A resolved credential is known to exist, but not known to be valid; the first `catalog -f` confirms validity.

## 4. SSH clone access (the user does this)

Missing repositories are cloned from `ssh://git@<git.host>/<project>/<repo>.git`. If clones fail with `Permission denied (publickey)` or `Host key verification failed`:

- Ask the user to test with `ssh -T git@github.com` (or their own host).
- Point them to their provider's docs for adding an SSH key. Never generate, copy, or upload keys for them without an explicit request.
- Accepting a host key the first time is the user's decision. Do not bypass it with `StrictHostKeyChecking=no`.

## 5. Smoke test (the agent does this)

```bash
batch-tool --version
batch-tool auth status
batch-tool catalog -f --style native      # confirms the credential works against the API
batch-tool labels --style native          # lists labels and aliases
batch-tool git status --style native <one-small-repo>
```

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `no credential found for ...` | No token source for that project | Step 3; check `batch-tool auth status`. |
| 401 or 403 from the API | Token is expired, lacks scopes, or belongs to the wrong account | The user rotates the token or fixes scopes; for several accounts, check `auth.owners` and use `auth status -v` digests. |
| `no projects configured` | `git.project` is unset or the config file was not found | Step 2; check stderr for `Using config file`. |
| A repository is missing from the selection | Stale catalog, wrong project, archived repository, or unwanted label | `catalog -f`, `labels -v`, qualify it as `project/repo`, or force it with `'+repo'`. |
| Unexpected repositories are selected | An alias or label is broader than expected | `labels -v <selectors>`, then add `'!exclude'`. |
| `skipping operation - main is the base branch` | Commit, push, or PR attempted on the default branch | Create a feature branch first with `git branch -b`. |
| `latest stash is not a batch-tool stash` | Someone else's stash is on top | Ask the user; use `--allow-any` only with their approval. |
| A pull fails because histories diverged | Local commits on the default branch | Ask the user; offer `--pull-strategy rebase`, `ff-only`, or `merge`. |
| The command hangs | TUI waiting in an interactive terminal | Use `--style native`, or add `--no-wait`. |
| GitHub secondary rate limit errors | Too many writes at once | `--sync` or a lower `--max-concurrency`. batch-tool already backs off automatically. |
| `exec` fails immediately | No `-y` with a non-interactive stdin, or the file is not executable | Once the command is authorized ([safety.md](safety.md)), add `-y`. `chmod +x` the script only with the user's approval. |
