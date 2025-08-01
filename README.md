# Batch Tool

This tool provides a suite of functionality for performing common tasks across multiple git repositories, including branch management and pull request creation.

## Features

- **Git Operations**: Create and push branches and commits, and manage history
- **Pull Request Management**: Create, edit, and merge pull requests (supports GitHub and Bitbucket)
- **Repository Catalog**: Cache repository metadata for quick matching and topical grouping
- **Make Target Execution**: Run make targets across multiple repositories
- **Flexible Configuration**: YAML-based configuration with repository aliases and default reviewers

## Getting Started

**TL;DR for new users:**
1. `make install` - Installs batch-tool to your system
2. Create a `batch-tool.yaml` file with your git provider and repositories
3. Run `batch-tool git status <repo-names>` to get started

For detailed setup instructions, see the sections below.

## Installation

### Pre-built Binaries

Download and unpack the binary for your platform from [the latest release](https://github.com/ryclarke/batch-tool/releases/latest).

### Build and Install from Source

For the fastest setup, use Make to install directly to your Go bin directory:

```bash
make install
```

This will automatically detect changes, build the tool, and install it to `$GOPATH/bin` (or `$HOME/go/bin` if `GOPATH` is not set).

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

## Quick Start

### 1. Install

If building from source, install the tool to your system:

```bash
make install
```

Make sure `$GOPATH/bin` (or `~/go/bin`) is in your `PATH`.

### 2. Configuration

Create a configuration file `batch-tool.yaml` in your working directory or user config directory (`~/.config`), or specify a file path manually with `--config`:

```yaml
git:
  provider: github
  host: github.com
  project: your-username-or-org
  default-branch: main # fallback for a repo with no default branch

repos:
  sort: true  # sort output alphabetically by repository name

  # don't match repos containing the following topics unless explicitly added
  skip-unwanted: true
  unwanted-labels: deprecated,poc
  
  # aliases act like custom topics for referencing and grouping repoistories
  aliases:
    myproject:
      - repo1
      - repo2
      - repo3

  # list of default reviewers to request for each repository
  reviewers:
    repo1:
      - reviewer1
      - reviewer2
```
The only required field is `git.project`, the rest of the configuration has safe default values.

### 3. Authentication

For repository discovery and pull request operations, you'll need to configure authentication:

- **GitHub**: Set up a personal access token
- **Bitbucket**: Configure API credentials

The authentication token should be provided via the `AUTH_TOKEN` environment variable (recommended) or the `auth-token` field in the batch-tool config file.

### 4. Basic Usage

Check the status of multiple repositories:
```bash
batch-tool git status repo1 repo2 repo3
```

Or use a Topic (or an alias defined in your config) by in:
```bash
batch-tool git status '~libraries'
```

To refer to an alias or topic, include `~` in the argument as seen above. To invert a match to *exclude* a repository or alias/topic, include a `!`. The `~all` alias

Examples:
```bash
# aliases:
#   repos: [repo1 repo2 repo3]
#
batch-tool git status '~repos' '!repo3' # matches repo1 and repo2 only
```

When using label matching and exclusion, ensure that all arguments are quoted properly to avoid improper shell expansion.

## Commands

### Git Operations

```bash
# Check status across repositories
batch-tool git status <repos...>

# Create new branches for each repository
batch-tool git branch -b "<branch-name>" <repos...>

# Checkout the default branches and pull any upstream changes
batch-tool git update <repos...>

# Show diff information in the working trees
batch-tool git diff <repos...>

# Commit and push changes
batch-tool git commit -m "commit message" <repos...>
```

### Pull Request Management

**Note**: Requires authentication token configuration.

```bash
# Create new pull requests
batch-tool pr new -t "PR Title" -d "Description" <repos...>

# Edit existing pull requests
batch-tool pr edit -t "New Title" -d "New Description" <repos...>

# Add requested reviewers by username
batch-tool pr edit -r reviewer1 -r reviewer2 <repos...>

# Merge all accepted pull requests
batch-tool pr merge <repos...>
```

### Miscellaneous

```bash
# Generate autocompletion script for the specified shell
batch-tool completion <bash|fish|powershell|zsh>

# Execute make targets
batch-tool make -t <make target> <repos...>

# Test repository filter rules against topics and local aliases
batch-tool labels <repos...>

# View local repository metadata
batch-tool catalog

# Run synchronously (useful for computationally-expensive operations)
batch-tool --sync <command> <repos...>
```

## Global Flags

- `--config string`: Specify config file (default: `batch-tool.yaml`)
- `--sync`: Execute commands synchronously instead of in parallel
- `--sort`: Sort repositories (default: true)
- `--skip-unwanted`: Skip repositories with unwanted labels (default: true)

## Configuration Reference

### Git Provider Settings

```yaml
git:
  provider: github | bitbucket
  host: github.com  # or your Bitbucket server
  project: your-org-or-username
  default-branch: main | develop
```

### Repository Settings

```yaml
repos:
  sort: true                   # Sort repository output
  skip-unwanted: true          # Skip repos with unwanted labels
  unwanted-labels:             # Labels to skip when skip-unwanted is true
    - deprecated
    - poc
    - archived
    
  aliases:                     # Group repositories under aliases
    frontend:
      - web-app
      - mobile-app
    backend:
      - api-server
      - worker-service
      
  reviewers:                   # Default reviewers per repository
    web-app:
      - frontend-team
    api-server:
      - backend-team
```

## Examples

### Daily Workflow

1. **Morning sync**: Update all repositories to latest
   ```bash
   batch-tool git update '~all'
   ```

2. **Create feature branch**: Start new work across multiple repos
   ```bash
   batch-tool git branch -b feature/new-feature '~frontend' '~backend'
   ```

3. **Check status**: See what's changed
   ```bash
   batch-tool git status '~frontend' '~backend'
   ```

4. **Create pull requests**: Submit your changes
   ```bash
   batch-tool pr new -t "Add new feature" -d "Detailed description" '~frontend' '~backend'
   ```

### Maintenance Tasks

1. **Run tests across projects**:
   ```bash
   batch-tool make -t test '~myproject'
   ```

2. **Format code**:
   ```bash
   batch-tool make -t format '~myproject'
   ```

3. **Synchronous operations** (when needed):
   ```bash
   batch-tool --sync make -t build '~myproject'
   ```

## Tips

- Use repository aliases in your config to group related repositories
- The `--sync` flag is useful for operations that must run sequentially
- Repository labels help organize and filter your catalog
- Default reviewers in config save time when creating pull requests
- The tool works from any directory - it will find repositories based on your configuration

## Troubleshooting

- **Authentication errors**: Ensure your API token is properly configured
- **Repository not found**: Check that repository names match your git provider
- **Sync issues**: Use `--sync` flag for operations that need to run sequentially
- **Config issues**: Verify your `batch-tool.yaml` syntax and paths

For more detailed help on any command, use:
```bash
batch-tool [command] --help
```
