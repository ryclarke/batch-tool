# Contributing to batch-tool

This is a short guide on how to contribute to the project.

## Submitting a pull request ##

If you find a bug that you'd like to fix, or a new feature that you'd like to implement then please submit a pull request via Github.

You'll need a Go environment set up with GOPATH set. See [the Go getting started docs](https://golang.org/doc/install) for more info.

Now in your terminal

```bash
git clone git@github.com:ryclarke/batch-tool.git
cd batch-tool
```

Make a branch to add your new feature

```bash
git checkout -b feature/my-new-feature
```

And get hacking.

Make sure you

  * Add documentation for a new feature
  * squash commits down to one per feature
  * rebase to the main branch `git rebase main`

When you are done with that

````bash
git push origin feature/my-new-feature
````

Go to the repoistory in Github and click [New pull request](https://github.com/ryclarke/batch-tool/compare).

### Code Style and Quality

This project uses [golangci-lint](https://golangci-lint.run/) with a curated set of linters
(see [.golangci.yml](.golangci.yml)). Before submitting a pull request, please verify that
your changes pass both lint and tests:

```bash
make lint    # runs golangci-lint with the project's configuration
make test    # runs the full test suite with -race
make cover   # runs tests with coverage reporting
```

Notes on conventions:

  * **Imports** are grouped via `gci` into standard, third-party, internal (`github.com/ryclarke/batch-tool`),
    and blank import sections — `make lint-fix` will sort them automatically.
  * **Viper access** must go through `config.Viper(ctx)` — direct imports of `github.com/spf13/viper`
    are restricted to the `config` package by `depguard`.
  * **Context propagation**: long-running operations (subprocesses, HTTP requests) must accept and
    honor a `context.Context`. Use `exec.CommandContext` and `http.NewRequestWithContext` rather
    than the non-context variants.
  * **Tests** prefer table-driven style and live in the same package (`package foo`, not `package foo_test`),
    using common helpers from `utils/testing` where applicable.

### Building from Source

Make provides several build targets:

```bash
# Install to your Go bin directory (recommended for new users)
make install

# Build for current platform only
make build

# Create release packages for all platforms
make release

# View all available targets
make help
```
