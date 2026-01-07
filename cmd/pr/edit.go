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
		Use:   "edit [-t <title>] [-d <description>] [-r <reviewer>]... [flags] <repository>...",
		Short: "Update existing pull requests",
		Long: `Update existing pull requests for the current branch.

This command updates PR details for existing pull requests using the SCM
provider API. You can update:
  - Title
  - Description
  - Reviewers
  - Draft status

The command:
  1. Finds the existing PR for the current branch
  2. Updates the specified fields
  3. Manages reviewers (add or replace based on flags)
  4. Displays the updated PR information

Partial Updates:
  You don't need to specify all fields. Only provided fields are updated:
  - Just title: Only title changes
  - Just reviewers: Only reviewers change
  - Multiple fields: All specified fields update

Branch Requirement:
  Must be on a feature branch with an existing PR.

Use Cases:
  - Update PR title or description
  - Add reviewers to existing PRs
  - Replace reviewer list
  - Correct PR information after creation`,
		Example: `  # Update PR title
  batch-tool pr edit -t "Updated title" repo1 repo2

  # Update description
  batch-tool pr edit -d "Updated description" repo1

  # Add reviewers and mark as ready for review
  batch-tool pr edit -r charlie --no-draft repo1

  # Replace all reviewers (only supported on Bitbucket)
  batch-tool pr edit -r alice -r bob --reset-reviewers repo1

  # Update multiple fields
  batch-tool pr edit -t "New title" -d "New desc" -r alice repo1

  # Update PRs for backend services
  batch-tool pr edit -t "Fix" ~backend`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrResetReviewers, cmd.Flags().Lookup(resetReviewersFlag))

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Edit)
		},
	}

	editCmd.Flags().Bool(resetReviewersFlag, false, "replace the reviewer list instead of appending to it")

	return editCmd
}

// Edit updates the pull request for the given repository.
func Edit(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return fmt.Errorf("failed to lookup branch for %s: %w", ch.Name(), err)
	}

	// Get project from repository metadata in catalog, fall back to default
	project := catalog.GetProjectForRepo(ctx, ch.Name())
	provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

	pr, err := provider.UpdatePullRequest(ch.Name(), branch, &scm.PROptions{
		Title:           viper.GetString(config.PrTitle),
		Description:     viper.GetString(config.PrDescription),
		Draft:           viper.GetBool(config.PrDraft),
		Reviewers:       lookupReviewers(ctx, ch.Name()),
		AppendReviewers: !viper.GetBool(config.PrResetReviewers),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(ch, "Updated pull request (#%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers)

	return nil
}
