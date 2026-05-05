# AGENTS.md

This file map for agents in this repo. Goal: less churn. Route change to package that owns behavior.

## What This Repository Is

See [CAVEMAN.md](CAVEMAN.md) for summary.

`batch-tool` is Cobra CLI for coordinating work across many git repos. Normal flow:

1. `main.go` calls `cmd.Execute()`.
2. `cmd/` wires flags, validates args, picks requested workflow.
3. `call.Do(...)` expands repo selectors, creates per-repo channels, manages concurrency, runs per-repo function.
4. `output/` renders progress and results in TUI or native output.
5. Domain packages like `catalog/`, `scm/`, `utils/` support selection, provider ops, shared helpers.

## Package Ownership

Route edits to closest owner. Do not patch symptom in higher layer.

### `cmd/`

- Owns Cobra command defs, help text, flag wiring, command-level validation.
- Must not hold business logic owned by `call/`, `catalog/`, `scm/`, `output/`.
- If bug shows only via CLI flag: check `cmd/` first, then move down to function that computes behavior.

### `call/`

- Owns fan-out execution across repos.
- Owns concurrency limits, cancel propagation, per-repo channel lifecycle, subprocess launch.
- If issue about batching, cancel, context cloning, concurrency, command flow: start here.

### `catalog/`

- Owns repo discovery, cached metadata, selector expansion, labels, aliases, repo matching.
- If behavior differs for `~label`, `!exclude`, `+force`, `~all`, unwanted labels, sorting: this owns fix.

### `output/`

- Owns presentation only: TUI, native output, catalog/label views, key handling, viewport behavior, print/wait flow.
- Do not move selection or execution policy into `output/` because bug appears there.

### `config/`

- Owns Viper init, config search paths, defaults, context-scoped config access.
- Only this package should import Viper directly for runtime config access.
- Need new config key: define here first, then wire into consumers.

### `scm/`, `scm/github/`, `scm/bitbucket/`, `scm/fake/`

- `scm/` owns provider-neutral contracts and shared models.
- Provider subpackages own API-specific request/response logic.
- `scm/fake/` is local test double. Keep behavior aligned with real providers.
- Network requests should use context-aware APIs.

### `utils/`

- Shared helpers across packages.
- Do not add domain policy here when owner package exists.
- If helper knows too much about git workflows, labels, SCM semantics, move it to proper owner.

## Documentation Ownership

- `README.md` is end-user docs only.
- `CONTRIBUTING.md` owns local build, release, test, lint, dev workflow guidance.
- `CAVEMAN.md` is concise purpose + mental model for fast agent orientation.
- If change affects install friction, command behavior, flags, config examples: update `README.md`.
- If change affects contributor setup, tooling, validation, release packaging: update `CONTRIBUTING.md`.

## Change Routing Rules

### CLI behavior changes

- Start in relevant command under `cmd/`.
- Step down to package that performs work before editing.
- Keep command help, examples, flags aligned with implementation.

### Repository selection bugs

- Inspect `catalog.RepositoryList`, label parsing, alias handling, related tests first.
- Do not patch output formatting to hide wrong selector semantics.

### Execution and cancellation bugs

- Inspect `call.Do`, `call.Exec`, context propagation, `output/` TUI interaction.
- Prefer one context-flow fix, not scattered local cancel logic.

### SCM / pull request bugs

- Start at `cmd/pr/` for flag binding and option construction.
- Then move to `scm/` contracts or provider impl that performs request.
- Keep fake provider consistent with real provider behavior when tests depend on it.

### Config bugs

- Add defaults and keys in `config/` first.
- Bind flags in `cmd/`.
- Read config through `config.Viper(ctx)` downstream.

## Validation Guidance

Prefer smallest validation that can falsify change.

- Full repo safety check with race detection: `make test`
- Lint and apply auto-fixes: `make lint-fix`
- Changed package only: run the directly affected package tests first when possible

Useful routing examples:

- `call/` or TUI cancellation changes: `go test ./call ./output ./cmd/...`
- `catalog/` selector changes: `go test ./catalog ./output`
- SCM changes: `go test ./scm/... ./cmd/pr`
- Config wiring changes: `go test ./config ./cmd/...`

## Testing Conventions

- Tests usually same-package, not external black-box test packages.
- Reuse `utils/testing` helpers instead of rebuilding common fixtures.
- When touching `scm/fake/`, ensure test expectations still reflect real provider contract.
- When touching selector behavior, validate repo results and user-facing label views depending on same parsing.

## Edit Style

- Prefer small, local edits over cross-package rewrites.
- Preserve existing public CLI and config behavior unless task explicitly changes it.
- If shorthand flag, config key, or output contract changes intentionally: update docs in same change.
- Do not broaden fix into unrelated cleanup unless cleanup required for correctness.
