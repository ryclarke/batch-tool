package pr

import (
	"context"
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/cmd/git"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	baseBranchFlag = "base-branch"
)

// addNewCmd initializes the pr new command
func addNewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "new [--draft] [-t <title>] [-d <description>] [-r <reviewer>]... [-b <base-branch>] <repository>...",
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
  batch-tool pr new -t "Fix bug" -d "Fixes issue #123" -r alice -r bob repo1 repo2

  # Create draft PR
  batch-tool pr new -t "WIP" --draft repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			config.Viper(cmd.Context()).BindPFlag(config.PrBaseBranch, cmd.Flags().Lookup(baseBranchFlag))

			return parseCommonPRFlags(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			viper := config.Viper(cmd.Context())

			buildPROptions(cmd)

			return call.Do(cmd, args, call.Wrap(git.ValidateBranch(viper.GetString(config.PrBaseBranch)), New))
		},
	}

	buildCommonPRFlags(newCmd)
	newCmd.Flags().StringP(baseBranchFlag, "b", "", "base branch for the pull request (default: repository default branch)")

	return newCmd
}

// New creates a new pull request for the given repository.
func New(ctx context.Context, ch output.Channel) error {
	provider, repoName, qualified := repoContext(ctx, ch.Name())

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return err
	}

	// load PR options from config
	opts := prOptions(ctx, qualified, false)
	if err := provider.CheckCapabilities(&opts); err != nil {
		return err
	}

	// get reviewers from config if not set via flags
	opts.Reviewers = lookupReviewers(ctx, qualified)
	opts.TeamReviewers = lookupTeamReviewers(ctx, qualified)

	pr, err := provider.OpenPullRequest(repoName, branch, &opts)
	if err != nil {
		return err
	}

	fmt.Fprint(ch, printPRInfo(pr, "New pull request", true))

	return nil
}

// lookupReviewers returns the list of individual reviewers for the given repository.
// It merges reviewers configured by repo name with those configured for any labels
// the repository belongs to (keyed by the label token, e.g. "~backend").
func lookupReviewers(ctx context.Context, qualified string) []string {
	viper := config.Viper(ctx)

	// Use the provided list of reviewers
	if revs := viper.GetStringSlice(config.PrReviewers); len(revs) > 0 {
		return revs
	}

	return lookupConfiguredReviewers(ctx, config.DefaultReviewers, qualified)
}

// lookupTeamReviewers returns the list of team reviewers for the given repository.
// It merges team reviewers configured by repo name with those configured for any labels
// the repository belongs to (keyed by the label token, e.g. "~backend").
func lookupTeamReviewers(ctx context.Context, qualified string) []string {
	viper := config.Viper(ctx)

	// Use the provided list of team reviewers
	if revs := viper.GetStringSlice(config.PrTeamReviewers); len(revs) > 0 {
		return revs
	}

	return lookupConfiguredReviewers(ctx, config.DefaultTeamReviewers, qualified)
}

// lookupConfiguredReviewers collects reviewers from the given config key for a repository.
// Repository entries may be keyed by either the project-qualified or bare name, and label
// entries are keyed by the label token (e.g. "~backend").
func lookupConfiguredReviewers(ctx context.Context, key, qualified string) []string {
	viper := config.Viper(ctx)

	reviewerMap := viper.GetStringMapStringSlice(key)
	tokenLabel := viper.GetString(config.TokenLabel)

	revs := mapset.NewSet(reviewerMap[qualified]...)

	// Accept bare repository names for backwards compatibility with unqualified configs
	if _, bare, found := strings.Cut(qualified, "/"); found {
		revs.Append(reviewerMap[bare]...)
	}

	// Merge in reviewers configured for any labels the repository belongs to
	for _, label := range catalog.GetLabelsForRepo(qualified) {
		revs.Append(reviewerMap[tokenLabel+label]...)
	}

	return revs.ToSlice()
}
