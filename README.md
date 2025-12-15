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
git clone git@github.com:ryclarke/batch-tool.git
cd batch-tool

make install
```

You'll need a Go environment set up with GOPATH set. See [the Go getting started docs](https://golang.org/doc/install) for more info.

This will automatically build the tool and install it to `$GOPATH/bin` (or `~/go/bin` if `GOPATH` is not set).

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

channels:
  output-style: bubbletea  # Use modern TUI interface

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

- **GitHub**: Set up a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- **Bitbucket**: Configure an [API token](https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/)

The authentication token should be provided via the `AUTH_TOKEN` environment variable (recommended) or the `auth-token` field in the batch-tool config file.

### 4. Basic Usage

Check the status of multiple repositories:
```bash
batch-tool git status repo1 repo2 repo3
```

Repositories can also be referenced by Github [Topics](https://github.com/topics) or Bitbucket [Labels](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-labels#api-group-labels):
```bash
batch-tool git status '~libraries'
```

You may also use the same syntax to refer to aliases defined locally in the config file.

- To refer to an alias or topic, include `~` in the argument as seen above.
- To invert a match to *exclude* a repository or alias/topic, include a `!`.
- To force selection of a repository or alias/topic, include a `+`.
  - This bypasses unwanted label filtering and ignores exclusions from other arguments.
  - If applied to an alias/topic, _every_ member will be included regardless of other filters.

-------------------------------

ℹ️ The `~all` alias is defined implicitly and refers to all discovered repositories for the configured namespace (user profile or organization).

-------------------------------

Example:
```bash
# repos.aliases:
#   myservice: [repo1 repo2 repo3 repo4]
#   deprecated: [repo4]
# repos.unwanted_labels: [deprecated]
#
batch-tool git status '~myservice' '!repo3' # matches repo1 and repo2 only
batch-tool git status '~myservice' '+repo4' # forces inclusion of repo4
batch-tool git status '+~myservice' '!~deprecated' # matches all 4 repos
```

⚠️ When using special characters for matching and exclusion, ensure that all arguments are quoted properly to avoid improper shell expansion.

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

# Execute arbitrary shell commands across repositories
## (DANGEROUS - use with caution) ##
batch-tool sh -c "command to execute" <repos...>

# Test repository filter rules against topics and local aliases
batch-tool labels <repos...>

# View local repository metadata
batch-tool catalog

# Run synchronously (useful for computationally-expensive operations)
batch-tool --sync <command> <repos...>
```

## Global Flags

- `--config string`: Specify config file (default: `batch-tool.yaml`)
- `--sync`: Execute commands synchronously (alias for `--max-concurrency=1`)
- `--max-concurrency int`: Maximum number of concurrent operations (default: number of logical CPUs)
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
  directory: /path/to/git/repos  # Base directory for repository clones
```

#### Repository Directory Structure

The `git.directory` option configures the base directory where repositories will be cloned. When specified, repositories are automatically organized in subdirectories that mirror the git provider's structure:

```
$GIT_DIRECTORY/
├── github.com/
│   ├── myorg/
│   │   ├── repo1/
│   │   ├── repo2/
│   │   └── repo3/
│   └── anothorg/
│       └── shared-repo/
└── bitbucket.example.com/
    └── myproject/
        └── api-service/
```

**Default behavior**: If not specified, defaults to `$GOPATH/src` if `GOPATH` is set, otherwise uses the current working directory.

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

### Output Style and Concurrency Settings

```yaml
channels:
  output-style: bubbletea    # Output style: "native" (default) or "bubbletea" (modern TUI)
  buffer-size: 100           # Channel buffer size for console output (default: 100)
  max-concurrency: 8         # Maximum concurrent operations (default: number of logical CPUs)
```

#### Output Styles

The `output-style` setting controls how command output is displayed:

- **`native`** (default): Traditional sequential output with repository headers. Output from each repository is batched and displayed after completion.
  
- **`bubbletea`**: Modern terminal UI with real-time updates, styled output, and progress indicators. Features include:
  - Live progress tracking with completion status
  - Styled repository names and status indicators
  - Real-time output streaming with full scrolling support
  - Color-coded errors and messages
  - Elapsed time display
  - Keyboard controls for navigation:
    - `↑`/`↓` or `j`/`k` - Scroll up/down one line
    - `PgUp`/`PgDn` or `b`/`f` or `Space` - Scroll by page
    - `Home`/`End` or `g`/`G` - Jump to top/bottom
    - `q` or `Ctrl+C` - Quit

**Note**: The bubbletea style provides a better experience for operations with many repositories or long-running commands, while the native style is more suitable for scripting and non-interactive use.

#### Concurrency Control

The `max-concurrency` setting controls how many repositories are processed simultaneously. This is useful for:

- **Resource-intensive operations**: Reduce concurrency to avoid overwhelming the system
- **Rate-limited APIs**: Prevent hitting API rate limits when working with pull requests
- **Network-bound operations**: Balance between speed and stability

**Examples:**
- `max-concurrency: 1` - Process repositories one at a time (equivalent to `--sync`)
- `max-concurrency: 5` - Conservative setting for API operations
- `max-concurrency: 20` - Aggressive setting for local git operations

**Tip**: The default concurrency is set to the number of logical CPUs on your system. Start with this default and adjust based on your specific use case and system capabilities.

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

4. **Execute custom commands** (use with caution):
```bash
# Example: Check Go version across repositories
batch-tool sh -c "go version" '~myproject'

# Example: Clean up temporary files
batch-tool sh -c "rm -f *.tmp" '~myproject'
```

## Tips

- Use repository aliases in your config to group related repositories
- The `--sync` flag is useful for operations that must run sequentially
- Repository labels help organize and filter your catalog
- Default reviewers in config save time when creating pull requests
- The tool works from any directory - it will find repositories based on your configuration
- **⚠️ Shell Command Safety**: The `sh` command is powerful but dangerous. It will prompt for confirmation before executing any command across multiple repositories. Use with extreme caution, especially with destructive commands.

## Troubleshooting

- **Authentication errors**: Ensure your API token is properly configured
- **Repository not found**: Check that repository names match your git provider
- **Sync issues**: Use `--sync` flag for operations that need to run sequentially
- **Config issues**: Verify your `batch-tool.yaml` syntax and paths

For more detailed help on any command, use:
```bash
batch-tool [command] --help
```
