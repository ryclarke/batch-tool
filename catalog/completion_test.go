package catalog

import (
	"io"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/spf13/cobra"
)

// checkCompletionCount checks that the number of completions matches the expected count
func checkCompletionCount(t *testing.T, completions []cobra.Completion, wantCount int) {
	t.Helper()
	if len(completions) != wantCount {
		t.Errorf("Expected %d completions, got %d: %v", wantCount, len(completions), completions)
	}
}

// checkCompletionContains verifies that all expected strings are in the completions
func checkCompletionContains(t *testing.T, completions []cobra.Completion, wantCompletions []string) {
	t.Helper()
	completionStrings := make([]string, len(completions))
	for i, c := range completions {
		completionStrings[i] = string(c)
	}
	completionSet := mapset.NewSet(completionStrings...)

	for _, expected := range wantCompletions {
		if !completionSet.Contains(expected) {
			t.Errorf("Expected completion '%s' not found in %v", expected, completionStrings)
		}
	}
}

// checkCompletionNotContains verifies that unwanted strings are not in the completions
func checkCompletionNotContains(t *testing.T, completions []cobra.Completion, unwantedCompletions []string) {
	t.Helper()
	completionStrings := make([]string, len(completions))
	for i, c := range completions {
		completionStrings[i] = string(c)
	}
	completionSet := mapset.NewSet(completionStrings...)

	for _, notExpected := range unwantedCompletions {
		if completionSet.Contains(notExpected) {
			t.Errorf("Unexpected completion '%s' found in %v", notExpected, completionStrings)
		}
	}
}

// checkDirective verifies the shell completion directive
func checkDirective(t *testing.T, directive cobra.ShellCompDirective, wantDirective cobra.ShellCompDirective) {
	t.Helper()
	if directive != wantDirective {
		t.Errorf("Expected directive %v, got %v", wantDirective, directive)
	}
}

// checkSetCardinality verifies the size of a set
func checkSetCardinality(t *testing.T, set mapset.Set[cobra.Completion], wantSize int) {
	t.Helper()
	if set.Cardinality() != wantSize {
		t.Errorf("Expected %d items in set, got %d: %v", wantSize, set.Cardinality(), set.ToSlice())
	}
}

// checkSetContains verifies a set contains an expected value
func checkSetContains(t *testing.T, set mapset.Set[cobra.Completion], wantValue string) {
	t.Helper()
	if !set.Contains(cobra.Completion(wantValue)) {
		t.Errorf("Expected set to contain '%s', got %v", wantValue, set.ToSlice())
	}
}

func TestCompletionFunc(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name              string
		toComplete        string
		wantCount         int
		wantDirective     cobra.ShellCompDirective
		wantCompletions   []string
		unwantCompletions []string
	}{
		{
			name:            "empty input returns all repos and labels",
			toComplete:      "",
			wantCount:       8, // 5 repos + 3 labels with token suffix
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
			wantCompletions: []string{"web-app", "mobile-app", "api-server", "worker", "tools", "frontend~", "backend~", "util~"},
		},
		{
			name:            "partial repo name",
			toComplete:      "web",
			wantCount:       1, // just web-app repo
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
			wantCompletions: []string{"web-app"},
		},
		{
			name:            "partial label name without token",
			toComplete:      "front",
			wantCount:       1,
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
			wantCompletions: []string{"frontend~"},
		},
		{
			name:              "label with token prefix",
			toComplete:        "~front",
			wantCount:         1,
			wantDirective:     cobra.ShellCompDirectiveNoFileComp,
			wantCompletions:   []string{"~frontend"},
			unwantCompletions: []string{"frontend~"},
		},
		{
			name:            "partial match multiple repos",
			toComplete:      "app",
			wantCount:       2,
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
			wantCompletions: []string{"web-app", "mobile-app"},
		},
		{
			name:              "label token suppresses repo suggestions",
			toComplete:        "~",
			wantCount:         3, // only labels, no repos
			wantDirective:     cobra.ShellCompDirectiveNoFileComp,
			wantCompletions:   []string{"~frontend", "~backend", "~util"},
			unwantCompletions: []string{"web-app", "mobile-app"},
		},
		{
			name:            "non-matching input",
			toComplete:      "xyz",
			wantCount:       0,
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
			wantCompletions: []string{},
		},
		{
			name:            "case-sensitive matching",
			toComplete:      "Web",
			wantCount:       0, // shouldn't match "web-app"
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
			wantCompletions: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Catalog = map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"mobile-app": {Name: "mobile-app"},
				"api-server": {Name: "api-server"},
				"worker":     {Name: "worker"},
				"tools":      {Name: "tools"},
			}

			Labels = map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
				"backend":  mapset.NewSet("api-server", "worker"),
				"util":     mapset.NewSet("tools"),
			}

			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetOut(io.Discard)

			completionFunc := CompletionFunc()
			completions, directive := completionFunc(cmd, []string{}, tt.toComplete)

			checkCompletionCount(t, completions, tt.wantCount)
			checkDirective(t, directive, tt.wantDirective)
			checkCompletionContains(t, completions, tt.wantCompletions)
			checkCompletionNotContains(t, completions, tt.unwantCompletions)
		})
	}
}

func TestCompletionFuncWithDifferentTokens(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name            string
		token           string
		toComplete      string
		wantCompletions []string
	}{
		{
			name:            "custom token @ recognized",
			token:           "@",
			toComplete:      "@my",
			wantCompletions: []string{"@mylabel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper := config.Viper(ctx)
			viper.Set(config.TokenLabel, tt.token)

			Catalog = map[string]scm.Repository{
				"repo1": {Name: "repo1"},
			}

			Labels = map[string]mapset.Set[string]{
				"mylabel": mapset.NewSet("repo1"),
			}

			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetOut(io.Discard)

			completionFunc := CompletionFunc()
			completions, _ := completionFunc(cmd, []string{}, tt.toComplete)

			checkCompletionContains(t, completions, tt.wantCompletions)
		})
	}
}

func TestAddLabelCompletion(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name       string
		label      string
		toComplete string
		wantCount  int
		wantValue  string
	}{
		{
			name:       "partial label name",
			label:      "frontend",
			toComplete: "front",
			wantCount:  1,
			wantValue:  "frontend~",
		},
		{
			name:       "label with token prefix",
			label:      "frontend",
			toComplete: "~front",
			wantCount:  1,
			wantValue:  "~frontend",
		},
		{
			name:       "exact match",
			label:      "backend",
			toComplete: "backend",
			wantCount:  1,
			wantValue:  "backend~",
		},
		{
			name:       "no match",
			label:      "frontend",
			toComplete: "xyz",
			wantCount:  0,
			wantValue:  "",
		},
		{
			name:       "empty input",
			label:      "test",
			toComplete: "",
			wantCount:  1,
			wantValue:  "test~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := mapset.NewSet[cobra.Completion]()
			addLabelCompletion(ctx, set, tt.label, tt.toComplete)

			checkSetCardinality(t, set, tt.wantCount)
			if tt.wantCount > 0 {
				checkSetContains(t, set, tt.wantValue)
			}
		})
	}
}

func TestAddRepoCompletion(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name       string
		repo       string
		toComplete string
		wantCount  int
	}{
		{
			name:       "partial repo name",
			repo:       "web-app",
			toComplete: "web",
			wantCount:  1,
		},
		{
			name:       "exact match",
			repo:       "api-server",
			toComplete: "api-server",
			wantCount:  1,
		},
		{
			name:       "no match",
			repo:       "web-app",
			toComplete: "mobile",
			wantCount:  0,
		},
		{
			name:       "empty input matches all",
			repo:       "tools",
			toComplete: "",
			wantCount:  1,
		},
		{
			name:       "substring match",
			repo:       "my-service-api",
			toComplete: "service",
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := mapset.NewSet[cobra.Completion]()
			addRepoCompletion(ctx, set, tt.repo, tt.toComplete)

			checkSetCardinality(t, set, tt.wantCount)
			if tt.wantCount > 0 {
				checkSetContains(t, set, tt.repo)
			}
		})
	}
}

func TestCompletionFuncEmptyCatalogAndLabels(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		wantCount int
	}{
		{
			name:      "empty catalog returns no completions",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Catalog = map[string]scm.Repository{}
			Labels = map[string]mapset.Set[string]{}

			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetOut(io.Discard)

			completionFunc := CompletionFunc()
			completions, directive := completionFunc(cmd, []string{}, "")

			checkCompletionCount(t, completions, tt.wantCount)
			checkDirective(t, directive, cobra.ShellCompDirectiveNoFileComp)
		})
	}
}

func TestCompletionFuncLabelSuppressesRepos(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name                string
		toComplete          string
		unwantedCompletions []string
	}{
		{
			name:                "label token suppresses repo suggestions",
			toComplete:          "~label",
			unwantedCompletions: []string{"repo1", "repo2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Catalog = map[string]scm.Repository{
				"repo1": {Name: "repo1"},
				"repo2": {Name: "repo2"},
			}

			Labels = map[string]mapset.Set[string]{
				"label1": mapset.NewSet("repo1"),
			}

			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetOut(io.Discard)

			completionFunc := CompletionFunc()
			completions, _ := completionFunc(cmd, []string{}, tt.toComplete)

			checkCompletionNotContains(t, completions, tt.unwantedCompletions)
		})
	}
}

func TestCompletionFuncMultipleMatches(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name            string
		toComplete      string
		minCount        int
		wantCompletions []string
	}{
		{
			name:            "multiple repos and label match",
			toComplete:      "service",
			minCount:        4,
			wantCompletions: []string{"service-api", "service-web", "service-worker", "service~"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Catalog = map[string]scm.Repository{
				"service-api":    {Name: "service-api"},
				"service-web":    {Name: "service-web"},
				"service-worker": {Name: "service-worker"},
				"frontend-app":   {Name: "frontend-app"},
			}

			Labels = map[string]mapset.Set[string]{
				"service": mapset.NewSet("service-api", "service-web", "service-worker"),
				"front":   mapset.NewSet("frontend-app"),
			}

			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.SetOut(io.Discard)

			completionFunc := CompletionFunc()
			completions, _ := completionFunc(cmd, []string{}, tt.toComplete)

			if len(completions) < tt.minCount {
				t.Errorf("Expected at least %d completions, got %d", tt.minCount, len(completions))
			}
			checkCompletionContains(t, completions, tt.wantCompletions)
		})
	}
}
