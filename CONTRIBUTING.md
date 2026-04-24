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
