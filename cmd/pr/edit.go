// Package pr provides pull request management commands for batch-tool.
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
	resetReviewersFlag = "reset-reviewers"
)

// addEditCmd initializes the pr edit command
func addEditCmd() *cobra.Command {
	editCmd := &cobra.Command{
		Use:   "edit [--draft|--no-draft] [-t <title>] [-d <description>] [-r <reviewer>]... [--reset-reviewers] <repository>...",
		Short: "Update existing pull requests",
		Long: `Update existing pull requests for the current branch.

This command updates PR details for existing pull requests using the SCM
provider API. You can update one or more of the following fields:
  - Title
  - Description
  - Reviewers
  - Draft status

Branch Requirement:
  Must be on a feature branch with an existing PR.`,
		Example: `  # Update PR title and description
  batch-tool pr edit -t "Updated title" -d "Updated description" repo1 repo2

  # Add reviewer and mark as ready for review
  batch-tool pr edit -r charlie --no-draft repo1

  # Replace existing reviewers with new list
  batch-tool pr edit -r alice -r bob --reset-reviewers repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrResetReviewers, cmd.Flags().Lookup(resetReviewersFlag))

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			buildPROptions(cmd)

			call.Do(cmd, args, Edit)
		},
	}

	editCmd.Flags().Bool(resetReviewersFlag, false, "replace the reviewer list instead of appending to it")

	return editCmd
}

// Edit updates the pull request for the given repository.
func Edit(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	// Get project from repository metadata in catalog, fall back to default
	project := catalog.GetProjectForRepo(ctx, ch.Name())
	provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return fmt.Errorf("failed to lookup branch for %s: %w", ch.Name(), err)
	}

	// load PR options from config
	opts := prOptions(ctx, ch.Name())

	pr, err := provider.UpdatePullRequest(ch.Name(), branch, &opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(ch, "Updated pull request (#%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers)

	return nil
}
