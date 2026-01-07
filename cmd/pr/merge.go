package pr

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	forceFlag = "force"
)

// addMergeCmd initializes the pr merge command
func addMergeCmd() *cobra.Command {
	mergeCmd := &cobra.Command{
		Use:   "merge [flags] <repository>...",
		Short: "Merge accepted pull requests",
		Long: `Merge approved pull requests for the current branch.

This command merges PRs using the SCM provider API. It:
  1. Finds the PR for the current branch
  2. Checks PR status (unless forced)
  3. Merges the PR if approved
  4. Displays merge confirmation

Safety Checks:
  By default, the command verifies the PR is in an approved/mergeable state
  before merging. This includes checking:
  - Required approvals received
  - CI/CD checks passed
  - No merge conflicts
  - Branch is up to date

Force Merge:
  Use --force (-f) to bypass status checks and merge anyway. This should be
  used with caution as it may merge PRs that haven't been properly reviewed
  or tested.

Merge Behavior:
  The actual merge behavior (squash, merge commit, rebase) is typically
  controlled by repository settings in your SCM provider.

Post-Merge:
  After merging, you typically want to:
  - Update local default branch: batch-tool git update <repo>
  - Clean up feature branch: git branch -d <branch>

Use Cases:
  - Merge approved PRs after review
  - Coordinate merges across multiple repos
  - Automate merge workflow
  - Merge PRs from CI/CD pipelines`,
		Example: `  # Merge approved PRs
  batch-tool pr merge repo1 repo2

  # Force merge without status checks
  batch-tool pr merge -f repo1

  # Merge all backend PRs
  batch-tool pr merge ~backend

  # Merge with synchronous execution
  batch-tool pr merge --sync repo1 repo2

  # Merge and update branches afterward
  batch-tool pr merge repo1 && batch-tool git update repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return config.Viper(cmd.Context()).BindPFlag(config.PrForceMerge, cmd.Flags().Lookup(forceFlag))
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Merge)
		},
	}

	mergeCmd.Flags().BoolP(forceFlag, "f", false, "attempt to merge without checking PR status")

	return mergeCmd
}

// Merge merges the pull request for the given repository.
func Merge(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	// Get project from repository metadata in catalog, fall back to default
	project := catalog.GetProjectForRepo(ctx, ch.Name())
	provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return err
	}

	pr, err := provider.MergePullRequest(ch.Name(), branch, viper.GetBool(config.PrForceMerge))
	if err != nil {
		return err
	}

	fmt.Fprintf(ch, "Merged pull request (#%d) %s\n", pr.Number, pr.Title)

	return nil
}
