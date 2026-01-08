package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	stashActionPush = "push"
	stashActionPop  = "pop"

	stashMessagePrefix   = "batch-tool"
	stashStateKeyPattern = "repos.stashed.%s" // repo name placeholder

	stashAllowAnyFlag = "allow-any"
)

func addStashCmd() *cobra.Command {
	stashCmd := &cobra.Command{
		Use:   "stash {push|pop [--allow-any]} <repository>...",
		Short: "Stash or restore uncommitted changes across repositories",
		Long: `Manage git stash across multiple repositories.

This command provides simple push/pop operations for git stash, allowing you
to temporarily save uncommitted changes before performing operations like
updating branches, then restore them afterward.

Operations:
  push    Save current uncommitted changes with a timestamped batch-tool message
  pop     Restore the most recently stashed batch-tool changes and remove from stack

Safety Features:
  - Push creates timestamped stash messages for tracking (batch-tool YYYY-MM-DDTHH:MM:SSZ)
  - Pop only restores stashes created by batch-tool (refuses to pop manual stashes)
  - Tracks stash state in session context to safely skip pop if nothing was stashed
  - Clean worktrees are reported (no error) when pushing with no changes
  - Returns error if stashed changes were expected but pop fails

Workflow Example:
  When you need to update branches but have uncommitted work:
    1. batch-tool git stash push ~backend    # Save changes with timestamp
    2. batch-tool git update ~backend        # Update branches
    3. batch-tool git stash pop ~backend     # Restore only batch-tool stashes

Note: Pop will only restore stashes with the 'batch-tool' prefix. Manual stashes
and stashes created by other tools are left untouched for safety.`,
		Example: `  # Stash changes in specific repositories
  batch-tool git stash push repo1 repo2

  # Restore stashed changes
  batch-tool git stash pop repo1 repo2

  # Stash all backend services before update
  batch-tool git stash push ~backend
  batch-tool git update ~backend
  batch-tool git stash pop ~backend`,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == stashActionPush && cmd.Flags().Changed(stashAllowAnyFlag) {
				return fmt.Errorf("the --%s flag is only valid with the 'pop' action", stashAllowAnyFlag)
			}

			return config.Viper(cmd.Context()).BindPFlag(config.GitStashAllowAny, cmd.Flags().Lookup(stashAllowAnyFlag))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := strings.ToLower(args[0])

			switch action {
			case stashActionPush:
				call.Do(cmd, args[1:], StashPush)
			case stashActionPop:
				call.Do(cmd, args[1:], call.Wrap(ValidateStash, StashPop))
			default:
				return fmt.Errorf("invalid stash action: %s (valid actions are 'push' or 'pop')", action)
			}

			return nil
		},
	}

	stashCmd.Flags().Bool(stashAllowAnyFlag, false, "Allow popping any stash, not just batch-tool stashes")

	return stashCmd
}

// StashPush saves uncommitted changes to the stash stack with a timestamped message.
// If the worktree is clean, it reports success without error. Saves whether changes
// were stashed for each repository to the viper context for later retrieval.
func StashPush(ctx context.Context, ch output.Channel) error {
	var stashed bool
	defer func() {
		// Ensure we mark the stash state even if an error occurs
		setStashState(ctx, ch.Name(), stashed)
	}()

	changed, err := lookupChanges(ctx, ch.Name())
	if err != nil {
		return fmt.Errorf("failed to check worktree status: %w", err)
	}

	if !changed {
		ch.WriteString("Nothing to stash - worktree is clean")
		return nil
	}

	// Create timestamped stash message
	message := fmt.Sprintf("%s %s", stashMessagePrefix, time.Now().Format(time.RFC3339))
	if err = call.Exec("git", "stash", "push", "-m", message)(ctx, ch); err != nil {
		return err
	}

	// Mark that we successfully stashed changes
	stashed = true

	return nil
}

// StashPop restores the most recently stashed changes if it's a batch-tool stash.
// Checks viper context to determine if changes were stashed by StashPush.
// Returns an error if changes were expected to be stashed but cannot be popped.
func StashPop(ctx context.Context, ch output.Channel) error {
	// Check if we stashed changes for this repo in this session
	if !getStashState(ctx, ch.Name()) {
		return nil
	}

	if err := call.Exec("git", "stash", "pop")(ctx, ch); err != nil {
		return fmt.Errorf("failed to pop stashed changes: %w", err)
	}

	// Clear the stashed flag after successful pop
	setStashState(ctx, ch.Name(), false)
	return nil
}

// ValidateStash checks if there is a batch-tool stash to pop.
// It returns an error if no suitable stash is found.
func ValidateStash(ctx context.Context, ch output.Channel) error {
	message, err := lookupStash(ctx, ch.Name())
	if err != nil {
		return fmt.Errorf("failed to lookup stash: %w", err)
	}

	if message == "" {
		return fmt.Errorf("No stash found to pop")
	}

	// Check if the latest stash is a batch-tool stash (unless overridden)
	// Note: git stash list format may include branch prefix like "On main: message"
	if !config.Viper(ctx).GetBool(config.GitStashAllowAny) && !strings.Contains(message, stashMessagePrefix) {
		return fmt.Errorf("Latest stash is not a batch-tool stash: %s", message)
	}

	setStashState(ctx, ch.Name(), true)
	return nil
}

// lookupChanges checks if the repository has uncommitted changes.
// This includes staged, unstaged, and untracked files.
func lookupChanges(ctx context.Context, repo string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = utils.RepoPath(ctx, repo)

	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func lookupStash(ctx context.Context, repo string) (string, error) {
	cmd := exec.Command("git", "stash", "list", "-n", "1", "--format=%s")
	cmd.Dir = utils.RepoPath(ctx, repo)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// setStashState stores whether changes were stashed for a repository in the viper context.
func setStashState(ctx context.Context, repo string, stashed bool) {
	key := fmt.Sprintf(stashStateKeyPattern, repo)
	config.Viper(ctx).Set(key, stashed)
}

// getStashState retrieves whether changes were stashed for a repository from the viper context.
func getStashState(ctx context.Context, repo string) bool {
	key := fmt.Sprintf(stashStateKeyPattern, repo)
	return config.Viper(ctx).GetBool(key)
}
