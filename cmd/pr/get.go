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

func addGetCmd() *cobra.Command {
	// getCmd represents the pr get command
	getCmd := &cobra.Command{
		Use:               "get <repository>...",
		Aliases:           []string{"list"},
		Short:             "Get pull request information",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(git.ValidateBranch, Get))
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

	ch.WriteString(fmt.Sprintf("(PR #%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers))
	if pr.Description != "" {
		ch.WriteString(fmt.Sprintln(pr.Description))
	}

	return nil
}
