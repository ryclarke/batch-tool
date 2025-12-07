package catalog

import (
	"context"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

// CompletionFunc provides shell completion suggestions based on the currently-known repositories and labels.
func CompletionFunc(completions ...cobra.Completion) cobra.CompletionFunc {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		ctx := cmd.Context()

		validCompletions := mapset.NewSet(completions...)
		for label := range Labels {
			// suggest label names matching the partial input
			addLabelCompletion(ctx, validCompletions, label, toComplete)
		}

		// suggest individual repository names if no label token is present
		if !strings.Contains(toComplete, config.Viper(ctx).GetString(config.TokenLabel)) {
			for repo := range Catalog {
				// suggest repository names matching the partial input
				addRepoCompletion(ctx, validCompletions, repo, toComplete)
			}
		}

		return validCompletions.ToSlice(), cobra.ShellCompDirectiveNoSpace // no space after completion, so the user can add other signal tokens if needed
	}
}

func addLabelCompletion(ctx context.Context, set mapset.Set[cobra.Completion], label, toComplete string) {
	token := config.Viper(ctx).GetString(config.TokenLabel)

	if toComplete != "" && strings.HasPrefix(token+label, toComplete) {
		// if the partial input matches a label *starting* with the token, suggest that
		set.Add(token + label)
	} else if strings.Contains(label, utils.CleanFilter(ctx, toComplete)) {
		// suggest label names matching the partial input, appending the label token
		set.Add(label + token)
	}
}

func addRepoCompletion(ctx context.Context, set mapset.Set[cobra.Completion], repo, toComplete string) {
	// suggest repository names matching the partial input
	if strings.Contains(repo, utils.CleanFilter(ctx, toComplete)) {
		set.Add(repo)
	}
}
