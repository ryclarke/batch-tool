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
	allReviewersFlag = "all-reviewers"
	baseBranchFlag   = "base-branch"
)

// addNewCmd initializes the pr new command
func addNewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "new [--draft] [-t <title>] [-d <description>] [-r <reviewer>]... [-a] [-b <base-branch>] <repository>...",
		Short: "Submit new pull requests",
		Long: `Create new pull requests for the current branch in each repository.

This command creates a new PR from the current branch to the default branch
(or specified base branch) using the SCM provider API.

Optional Information:
  - Title: PR title (defaults to the feature branch name)
  - Description: PR body/description text
  - Reviewers: One or more reviewers to assign
  - Base Branch: Target branch for the PR (defaults to repo default branch)

Branch Validation:
  PRs cannot be created from the default branch. Ensure you're not on
  the default branch before running this command.`,
		Example: `  # Create PR with description and multiple reviewers
  batch-tool pr new -t "Fix bug" -d "Fixes issue #123" -a -r alice -r bob repo1 repo2

  # Create draft PR
  batch-tool pr new -t "WIP" --draft repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrAllReviewers, cmd.Flags().Lookup(allReviewersFlag))
			viper.BindPFlag(config.PrBaseBranch, cmd.Flags().Lookup(baseBranchFlag))

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			viper := config.Viper(cmd.Context())

			buildPROptions(cmd)

			call.Do(cmd, args, call.Wrap(git.ValidateBranch(viper.GetString(config.PrBaseBranch)), New))
		},
	}

	newCmd.Flags().BoolP(allReviewersFlag, "a", false, "use all provided reviewers for a new PR (default: only the first)")
	newCmd.Flags().StringP(baseBranchFlag, "b", "", "base branch for the pull request (default: repository default branch)")

	return newCmd
}

// New creates a new pull request for the given repository.
func New(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	// Get project from repository metadata in catalog, fall back to default
	project := catalog.GetProjectForRepo(ctx, ch.Name())
	provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return err
	}

	// load PR options from config
	opts := prOptions(ctx, ch.Name())

	// get reviewers from config if not set via flags
	opts.Reviewers = lookupReviewers(ctx, ch.Name())

	pr, err := provider.OpenPullRequest(ch.Name(), branch, &opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(ch, "New pull request (#%d) %s %v\n", pr.Number, pr.Branch, pr.Reviewers)

	return nil
}

// lookupReviewers returns the list of reviewers for the given repository
func lookupReviewers(ctx context.Context, name string) []string {
	viper := config.Viper(ctx)

	// Use the provided list of reviewers
	if revs := viper.GetStringSlice(config.PrReviewers); len(revs) > 0 {
		return revs
	}

	// Use default reviewers for the given repository
	revs := viper.GetStringMapStringSlice(config.DefaultReviewers)[name]

	// Only use the first reviewer from the default list unless allReviewers is set
	if !viper.GetBool(config.PrAllReviewers) && len(revs) > 1 {
		revs = []string{revs[0]}
	}

	return revs
}
