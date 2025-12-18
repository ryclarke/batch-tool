package catalog

import (
	"context"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// NoCompletion provides a no-op completion function to disable file completions for interstitial commands.
func NoCompletion() cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompletionFunc provides shell completion suggestions based on the currently-known repositories and labels.
func CompletionFunc() cobra.CompletionFunc {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		ctx := cmd.Context()
		viper := config.Viper(ctx)

		validCompletions := mapset.NewSet[cobra.Completion]()
		for label := range Labels {
			// suggest label names matching the partial input
			addLabelCompletion(ctx, validCompletions, label, toComplete)
		}

		// suggest individual repository names if no label token is present
		if !strings.Contains(toComplete, viper.GetString(config.TokenLabel)) {
			for repo := range Catalog {
				// suggest repository names matching the partial input
				addRepoCompletion(ctx, validCompletions, repo, toComplete)
			}
		}

		// Only apply NoSpace directive when we have batch-tool specific completions (repos/labels)
		// Otherwise use default behavior to allow normal shell completions
		if validCompletions.Cardinality() > 0 {
			return validCompletions.ToSlice(), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
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
