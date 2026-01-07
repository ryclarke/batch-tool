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

func addGetCmd() *cobra.Command {
	// getCmd represents the pr get command
	getCmd := &cobra.Command{
		Use:     "get <repository>...",
		Aliases: []string{"list"},
		Short:   "Get pull request information",
		Long: `Retrieve and display pull request information from the SCM provider.

This command fetches PR details for the current branch in each specified
repository, including:
  - PR number
  - Title
  - Description
  - Reviewers
  - Status (if available)

The command uses the SCM provider API to fetch real-time PR information.
It requires:
  - An active pull request for the current branch
  - Valid authentication token
  - The repository to be tracked in your catalog

Branch Requirement:
  The command must be run when repositories are on a feature branch (not the
  default branch). It looks up the PR associated with the current branch.

Use Cases:
  - Check PR status before merging
  - Verify PR details and reviewers
  - Get PR numbers for reference
  - Review PR descriptions across repos`,
		Example: `  # Get PR info for specific repositories
  batch-tool pr get repo1 repo2

  # Get PR info for all backend services
  batch-tool pr get ~backend

  # Using list alias
  batch-tool pr list repo1 repo2

  # Get PR with native output
  batch-tool pr get -o native repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Get)
		},
	}

	return getCmd
}

// Get retrieves and displays the pull request information for the given repository.
func Get(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return fmt.Errorf("failed to lookup branch for %s: %w", ch.Name(), err)
	}

	// Get project from repository metadata in catalog, fall back to default
	project := catalog.GetProjectForRepo(ctx, ch.Name())
	provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

	pr, err := provider.GetPullRequest(ch.Name(), branch)
	if err != nil {
		return fmt.Errorf("failed to get pull request for %s: %w", ch.Name(), err)
	}

	fmt.Fprintf(ch, "(PR #%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers)
	if pr.Description != "" {
		fmt.Fprintln(ch, pr.Description)
	}

	return nil
}
