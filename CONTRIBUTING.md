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
