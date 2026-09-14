package pr

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

func addGetCmd() *cobra.Command {
	// getCmd represents the pr get command
	getCmd := &cobra.Command{
		Use:     "get <repository>...",
		Aliases: []string{"list"},
		Short:   "Get pull request information",
		Long: `Retrieve and display pull request information from the SCM provider.

The command uses the SCM provider API to fetch real-time PR information.
It requires:
  - An active pull request for the current branch
  - Valid authentication token
  - The repository to be tracked in your catalog

Branch Requirement:
  The command must be run when repositories are on a feature branch (not the
  default branch). It looks up the PR associated with the current branch.`,
		Example: `  # Get PR info for specific repositories
  batch-tool pr get repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return call.Do(cmd, args, Get)
		},
	}

	return getCmd
}

// Get retrieves and displays the pull request information for the given repository.
func Get(ctx context.Context, ch output.Channel) error {
	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return fmt.Errorf("failed to lookup branch for %s: %w", ch.Name(), err)
	}

	provider, repoName, _ := repoContext(ctx, ch.Name())

	pr, err := provider.GetPullRequest(repoName, branch)
	if err != nil {
		return fmt.Errorf("failed to get pull request for %s: %w", repoName, err)
	}

	fmt.Fprint(ch, printPRInfo(pr, "", true))

	return nil
}
