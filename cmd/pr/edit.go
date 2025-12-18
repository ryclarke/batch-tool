// Package pr provides pull request management commands for batch-tool.
package pr

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/cmd/git"
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
		Use:               "edit <repository>...",
		Short:             "Update existing pull requests",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrResetReviewers, cmd.Flags().Lookup(resetReviewersFlag))

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(git.ValidateBranch, Edit))
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

	pr, err := provider.UpdatePullRequest(ch.Name(), branch, viper.GetString(config.PrTitle), viper.GetString(config.PrDescription),
		lookupReviewers(ctx, ch.Name()), !viper.GetBool(config.PrResetReviewers))
	if err != nil {
		return err
	}

	fmt.Fprintf(ch, "Updated pull request (#%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers)

	return nil
}
