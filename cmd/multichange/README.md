# Multi-Change Command

The `multichange` command provides functionality to apply tested changes from one repository to multiple repositories. This enables safe, controlled rollout of changes across your repository ecosystem.

## Overview

The workflow is designed around the following process:

1. **Make changes in a source repository** - Apply your changes to a repository and test them
2. **Test and validate** - Ensure the changes work correctly 
3. **Extract changes** - Use the `extract` command to capture the file differences
4. **Apply to other repositories** - Use the `apply` command to roll out changes to target repositories

## Commands

### `extract`

Extracts file changes from a specified git repository and creates old/new file pairs.

```bash
# Extract uncommitted changes (working directory vs HEAD)
batch-tool multichange extract --repo-path /path/to/repo --output-dir ./changes

# Extract changes between main branch and current HEAD
batch-tool multichange extract --repo-path /path/to/repo --output-dir ./changes --base-branch main --git-ref HEAD

# Extract changes between two specific commits
batch-tool multichange extract --repo-path /path/to/repo --output-dir ./changes --base-branch abc123 --git-ref def456
```

**Options:**
- `--repo-path` (required): Path to the git repository with changes
- `--output-dir` (required): Directory to save extracted file pairs
- `--base-branch`: Base branch/commit to compare from (default: "HEAD")
- `--git-ref`: Git reference to compare to (leave empty for working directory changes)

### `apply`

Applies changes that have been tested in a source repository to other repositories.

```bash
# Apply changes to repositories with specific labels
batch-tool multichange apply --changes-dir ./changes --target-labels backend-services

# Apply changes to specific repositories
batch-tool multichange apply --changes-dir ./changes --target-repos repo1,repo2,repo3

# Dry run to see what would be changed
batch-tool multichange apply --changes-dir ./changes --target-labels backend-services --dry-run

# Use enhanced diff-based matching for more flexible change application
batch-tool multichange apply --changes-dir ./changes --diff-based-match --target-labels backend-services

# Use both diff-based and partial matching for maximum flexibility
batch-tool multichange apply --changes-dir ./changes --diff-based-match --partial-match --target-labels backend-services
```

**Options:**
- `--changes-dir` (optional): Directory containing old/new file pairs (default: $GOPATH/src/changes)
- `--target-repos`: Specific repositories to apply changes to
- `--target-labels`: Repository labels to apply changes to
- `--dry-run`: Show what would be changed without making actual changes
- `--partial-match`: Enable partial matching for code blocks when full file doesn't match
- `--diff-based-match`: Enable enhanced diff-based matching (more precise than partial matching)
- `--canary-label`: Label identifying the canary service (default: "canary")

## Matching Strategies

The `apply` command supports three matching strategies:

### 1. Exact Matching (Default)
Files must match exactly (by content hash) between the `.old` file and the target file. This is the safest approach but requires identical file content.

### 2. Partial Matching (`--partial-match`)
When exact matching fails, the tool attempts to find and replace specific code blocks within files. This works well for structured code files but may miss some changes.

### 3. Diff-Based Matching (`--diff-based-match`) **NEW**
This enhanced approach parses the actual differences between `.old` and `.new` files and applies only the specific line changes to target files. It can handle cases where:

- Files have similar but not identical content
- Additional lines exist before or after the changed sections
- Different formatting or spacing exists in target files
- Multiple small changes need to be applied within the same file

The diff-based matching strategy:
1. Generates a unified diff between `.old` and `.new` files
2. Parses the diff to identify exactly which lines changed
3. Finds matching locations in target files using context-aware matching
4. Applies only the specific line changes while preserving surrounding content

**Example scenario**: You want to change a Docker label from `'docker-rootless || docker20X'` to `'docker20X'` in multiple Jenkinsfiles. The files may have different comments, additional stages, or environment variables, but you only want to update that specific label line.

With diff-based matching, the tool will:
- Identify that only one specific line needs to change
- Find that line in each target file regardless of surrounding content
- Apply only that change while preserving everything else

You can use both strategies together: `--diff-based-match --partial-match` to try diff-based first and fall back to partial matching if needed.

### `cleanup`

Removes all files from the changes directory to clean up after applying changes or prepare for a new extraction.

```bash
# Remove all extracted changes from standard location
batch-tool multichange cleanup

# Remove all extracted changes from custom location
batch-tool multichange cleanup --changes-dir ./changes
```

**Options:**
- `--changes-dir` (optional): Directory containing old/new file pairs to remove (default: $GOPATH/src/changes)

The cleanup command will:
1. Remove all files and subdirectories in the changes directory
2. Keep the changes directory itself (empty)
3. Report what was removed

## File Structure

By default, all commands use the standard changes directory at `$GOPATH/src/changes`. The `extract` command creates a directory structure with old/new file pairs:

```
$GOPATH/src/changes/
├── path/to/file1.old
├── path/to/file1.new
├── path/to/file2.old
├── path/to/file2.new
└── deeply/nested/file3.old
└── deeply/nested/file3.new
```

The `apply` command processes these pairs by:
1. Comparing each `.old` file with the corresponding file in target repositories
2. If they match exactly (by content hash), applying the `.new` file content
3. Reporting what changes were made or skipped

## Safety Features

- **Content verification**: Files are only updated if the existing content exactly matches the `.old` file
- **Hash comparison**: Uses SHA256 hashes to ensure exact content matches
- **Dry run mode**: Preview changes before applying them
- **Detailed reporting**: Shows exactly which files were changed, skipped, or failed
- **Canary exclusion**: By default, canary repositories are excluded from the target list

## Example Workflow

### Simple Workflow (Working Directory Changes) - Using Standard Location

```bash
# 1. Make changes in your source repository
cd /path/to/source-repo
# ... make your changes ...
# (don't commit yet - work with uncommitted changes)

# 2. Extract the uncommitted changes to standard location
batch-tool multichange extract --repo-path .

# 3. Preview what would be applied
batch-tool multichange apply --target-labels production-services --dry-run

# 4. Apply the changes
batch-tool multichange apply --target-labels production-services

# 5. Clean up after successful application
batch-tool multichange cleanup
```

### Advanced Workflow (Committed Changes) - Using Standard Location

```bash
# 1. Make and commit changes in your source repository
cd /path/to/source-repo
# ... make your changes ...
git add .
git commit -m "Update configuration for new feature"

# 2. Extract the changes between commits to standard location
batch-tool multichange extract --repo-path . --base-branch main --git-ref HEAD

# 3. Preview what would be applied
batch-tool multichange apply --target-labels production-services --dry-run

# 4. Apply the changes
batch-tool multichange apply --target-labels production-services

# 5. Clean up the extracted changes
batch-tool multichange cleanup
```

### Custom Directory Workflow (if needed)

```bash
# Extract to custom location
batch-tool multichange extract --repo-path . --output-dir ./config-changes

# Apply from custom location
batch-tool multichange apply --changes-dir ./config-changes --target-labels production-services

# Clean up custom location
batch-tool multichange cleanup --changes-dir ./config-changes
```

## Configuration

The command uses the existing batch-tool configuration for:
- Repository catalog and labels
- Git directory structure
- Target repository identification

Make sure your `batch-tool.yaml` configuration includes appropriate labels for your repositories.

## Error Handling

The command will skip files that:
- Don't exist in the target repository
- Have content that doesn't match the expected `.old` version
- Cannot be read or written due to permissions

Each repository is processed independently, so failures in one repository won't prevent changes in others.
