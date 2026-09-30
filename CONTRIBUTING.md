# Contributing to batch-tool

This guide covers local development, validation, and pull request expectations for contributors.

## Prerequisites

- Go 1.26 or later
- Make
- Git

## Local Setup

Clone the repository and create a feature branch:

```bash
git clone git@github.com:ryclarke/batch-tool.git
cd batch-tool
git checkout -b feature/my-change
```

Install the development toolchain used by the project:

```bash
make deps
```

This installs:

- `gotestsum` for test output
- `goreleaser` for build and release packaging
- `golangci-lint` for linting and formatting enforcement

## Common Development Commands

```bash
make test      # run the test suite with -race
make cover     # run tests with coverage output
make lint      # run golangci-lint
make lint-fix  # apply safe lint-driven fixes
make build     # build the current platform binary via goreleaser
make install   # install batch-tool into your Go bin directory
make release   # build release artifacts for all configured platforms
make help      # list all available targets
```

## Code Conventions

- Keep `cmd/` focused on Cobra command wiring, flag binding, and argument validation.
- Keep execution behavior in `call/`, not in command handlers.
- Keep rendering and interaction logic in `output/`.
- Access Viper only through `config.Viper(ctx)`. Direct imports of `github.com/spf13/viper` are restricted to the `config` package.
- Long-running subprocesses and HTTP requests should honor `context.Context` by using `exec.CommandContext` and `http.NewRequestWithContext`.
- Prefer same-package tests (`package foo`) and reuse helpers from `utils/testing`.
- Add or update docs when behavior, flags, or configuration expectations change.

## Agent Skill

[`skills/batch-tool/`](skills/batch-tool/) is an [Agent Skills](https://agentskills.io/specification) package that is distributed straight from this repository. Keep it in sync with the CLI:

- If you add, remove, or rename a command or flag, or change its default, update `skills/batch-tool/references/commands.md` in the same pull request.
- If you change how destructive a command is (for example a new discard or force path, or a new config key that escalates behavior), update the risk tiers in `references/safety.md`.
- If you change release archive names or the checksum format in `.goreleaser.yaml`, update `skills/batch-tool/scripts/install.sh`.
- Do not change `metadata.version` in feature or skill pull requests. The release workflow owns that field and keeps it equal to the latest SemVer release tag.

Validate locally:

```bash
pipx run --spec skills-ref==0.1.1 agentskills validate skills/batch-tool
shellcheck -s sh skills/batch-tool/scripts/install.sh
sh skills/batch-tool/scripts/install.sh --check
npx skills add ./ --list   # confirms the skill is discoverable
```

## Release Process

Releases are prepared and published by the `Release` GitHub Actions workflow. A maintainer starts it with an exact SemVer version without a leading `v`, for example `1.2.0` or `1.2.0-rc.1`. The version covers every change since the previous release, so the maintainer decides whether it is a major, minor, patch, or prerelease increment. The workflow uses npm's `semver` CLI for version validation and ordering, and `yq` for the skill metadata update.

The workflow validates the requested version and the current `main` branch before asking for approval through the protected `release` Environment. After approval, it:

1. Updates `skills/batch-tool/SKILL.md`.
2. Commits the version change to `main`.
3. Creates the matching annotated `v<version>` tag.
4. Pushes the commit and tag atomically.
5. Runs GoReleaser to publish the GitHub release.

## AI-Assisted Contributions

AI-assisted work is allowed, but only with direct maintainer oversight.

By submitting a change, you confirm that you reviewed and understood the final diff, validated behavior against expectations, and accept responsibility for correctness and safety.

## Pull Request Workflow

Before opening a pull request:

1. Rebase onto `main`.
2. Run `make lint` and `make test`.
3. Update tests for behavior changes.
4. Update user-facing docs if the CLI, config, or workflows changed.

Then push your branch:

```bash
git push origin feature/my-change
```

Open a pull request at [github.com/ryclarke/batch-tool/compare](https://github.com/ryclarke/batch-tool/compare).

## Review Checklist

- The change stays in the owning package instead of duplicating behavior elsewhere.
- Tests cover new behavior or changed edge cases.
- New config keys, flags, or workflow changes are reflected in documentation.
