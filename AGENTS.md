# AGENTS.md

This file is a starting map for agents working in this repository. Its purpose is to reduce churn by routing changes to the package that actually owns the behavior.

## What This Repository Is

`batch-tool` is a Cobra-based CLI for coordinating work across many git repositories. The normal execution path is:

1. `main.go` calls `cmd.Execute()`.
2. `cmd/` binds flags, validates arguments, and selects the requested workflow.
3. `call.Do(...)` expands repository selectors, creates per-repo channels, manages concurrency, and runs the per-repository function.
4. `output/` renders progress and results in TUI or native form.
5. Domain packages such as `catalog/`, `scm/`, and `utils/` support selection, provider operations, and shared helpers.

## Package Ownership

Route edits to the closest owner instead of patching around symptoms in a higher layer.

### `cmd/`

- Owns Cobra command definitions, help text, flag wiring, and command-level validation.
- Should not accumulate business logic that belongs in `call/`, `catalog/`, `scm/`, or `output/`.
- If a bug is only reproduced through a CLI flag, inspect `cmd/` first, then step to the lower-level function that actually computes behavior.

### `call/`

- Owns fan-out execution across repositories.
- Owns concurrency limits, cancellation propagation, per-repo channel lifecycle, and subprocess launching.
- If the issue is about batching, cancellation, cloning, concurrency, or command execution flow, start here.

### `catalog/`

- Owns repository discovery, cached metadata, selector expansion, labels, aliases, and repo matching.
- If behavior differs for `~label`, `!exclude`, `+force`, `~all`, unwanted labels, or sorting, this is the primary owner.

### `output/`

- Owns presentation only: TUI, native output, catalog/label displays, key handling, viewport behavior, and print/wait flow.
- Do not move selection or execution policy into `output/` just because the bug is visible there.

### `config/`

- Owns Viper initialization, config search paths, defaults, and context-scoped config access.
- This is the only package that should import Viper directly for runtime config access.
- If you need a new config key, define it here first and then wire it into consumers.

### `scm/`, `scm/github/`, `scm/bitbucket/`, `scm/fake/`

- `scm/` owns provider-neutral contracts and shared models.
- Provider subpackages own API-specific request/response logic.
- `scm/fake/` is the local test double and should stay behaviorally aligned with the real providers.
- Network requests should use context-aware APIs.

### `utils/`

- Shared helpers used across packages.
- Avoid adding domain policy here when an owning package already exists.
- If a helper starts knowing too much about git workflows, labels, or SCM semantics, it probably belongs somewhere else.

## Documentation Ownership

- `README.md` is end-user documentation only.
- `CONTRIBUTING.md` owns local build, release, test, lint, and development workflow guidance.
- If a change affects install friction, command behavior, flags, or config examples, update `README.md`.
- If a change affects contributor setup, tooling, validation, or release packaging, update `CONTRIBUTING.md`.

## Change Routing Rules

### CLI behavior changes

- Start in the relevant command under `cmd/`.
- Step down to the package that actually performs the work before editing.
- Keep command help, examples, and flags aligned with the implementation.

### Repository selection bugs

- Inspect `catalog.RepositoryList`, label parsing, alias handling, and related tests first.
- Do not patch output formatting to compensate for incorrect selector semantics.

### Execution and cancellation bugs

- Inspect `call.Do`, `call.Exec`, context propagation, and `output/` TUI interaction.
- Prefer fixing context flow once instead of sprinkling local cancel logic across commands.

### SCM / pull request bugs

- Start at `cmd/pr/` for flag binding and option construction.
- Then move to `scm/` contracts or the provider implementation that performs the request.
- Keep the fake provider consistent with real-provider behavior when tests depend on it.

### Config bugs

- Add defaults and keys in `config/` first.
- Bind flags in `cmd/`.
- Read config through `config.Viper(ctx)` downstream.

## Validation Guidance

Prefer the smallest validation that can falsify the change.

- Full repo safety check with race detection: `make test`
- Lint and apply auto-fixes: `make lint-fix`
- Changed package only: run the directly affected package tests first when possible

Useful routing examples:

- `call/` or TUI cancellation changes: `go test ./call ./output ./cmd/...`
- `catalog/` selector changes: `go test ./catalog ./output`
- SCM changes: `go test ./scm/... ./cmd/pr`
- Config wiring changes: `go test ./config ./cmd/...`

## Testing Conventions

- Tests are generally same-package tests, not external black-box test packages.
- Reuse `utils/testing` helpers instead of rebuilding common fixtures.
- When touching `scm/fake/`, make sure test expectations still reflect the real provider contract.
- When touching selector behavior, validate both repository results and any user-facing label views that depend on the same parsing.

## Edit Style

- Prefer small, local edits over cross-package rewrites.
- Preserve existing public CLI and config behavior unless the task explicitly changes it.
- If a shorthand flag, config key, or output contract changes intentionally, update the documentation in the same change.
- Do not broaden a fix into unrelated cleanup unless the cleanup is required to make the change correct.
