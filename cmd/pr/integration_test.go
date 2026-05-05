package pr

import (
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

func TestPRIntegrationWithFakeProvider(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Configure for fake provider
	viper.Set(config.GitProvider, "fake")
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.AuthToken, "fake-token")

	// Register fake provider with test data
	scm.Register("fake-pr-test", func(_ context.Context, project string) scm.Provider {
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	// Update the provider for the test
	viper.Set(config.GitProvider, "fake-pr-test")

	// Verify subcommands are registered
	t.Run("NewCommand", func(t *testing.T) {
		if addNewCmd() == nil {
			t.Fatal("addNewCmd() returned nil")
		}
	})

	t.Run("EditCommand", func(t *testing.T) {
		if addEditCmd() == nil {
			t.Fatal("addEditCmd() returned nil")
		}
	})

	t.Run("MergeCommand", func(t *testing.T) {
		if addMergeCmd() == nil {
			t.Fatal("addMergeCmd() returned nil")
		}
	})
}

func TestPRCommandFlags(t *testing.T) {
	loadFixture(t)

	tests := []struct {
		name      string
		cmdFunc   func() *cobra.Command
		flagName  string
		shorthand string
	}{
		// Common PR CRUD flags are local to new/edit commands
		{"New_Title", addNewCmd, "title", "t"},
		{"New_Description", addNewCmd, "description", "d"},
		{"New_Reviewer", addNewCmd, "reviewer", "r"},
		{"New_TeamReviewer", addNewCmd, "team-reviewer", "R"},
		{"New_Draft", addNewCmd, "draft", ""},
		{"New_NoDraft", addNewCmd, "no-draft", ""},
		{"Edit_Title", addEditCmd, "title", "t"},
		{"Edit_Description", addEditCmd, "description", "d"},
		{"Edit_Reviewer", addEditCmd, "reviewer", "r"},
		{"Edit_TeamReviewer", addEditCmd, "team-reviewer", "R"},
		{"Edit_Draft", addEditCmd, "draft", ""},
		{"Edit_NoDraft", addEditCmd, "no-draft", ""},

		// pr new — local flag
		{"New_BaseBranch", addNewCmd, "base-branch", "b"},

		// pr edit — local flag
		{"Edit_ResetReviewers", addEditCmd, "reset-reviewers", ""},

		// pr merge — persistent bool pair + local string flag
		{"Merge_Force", addMergeCmd, "force", "f"},
		{"Merge_Check", addMergeCmd, "check", ""},
		{"Merge_Method", addMergeCmd, "method", "m"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// cmd.Flag() checks both local and persistent flags
			flag := test.cmdFunc().Flag(test.flagName)
			if flag == nil {
				t.Fatalf("Flag %q not found", test.flagName)
			}

			if flag.Shorthand != test.shorthand {
				t.Errorf("Flag %q: expected shorthand %q, got %q", test.flagName, test.shorthand, flag.Shorthand)
			}
		})
	}
}

func TestValidatePRConfig(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Test with missing configuration
	viper.Set(config.GitProvider, "")
	viper.Set(config.GitProject, "")
	viper.Set(config.AuthToken, "")

	// Test that the command structure is correct
	cmd := addNewCmd()
	if cmd.RunE != nil {
		t.Log("RunE function is set as expected")
	}
}

func TestPRCommandValidation(t *testing.T) {
	tests := []struct {
		name     string
		cmdFunc  func() *cobra.Command
		args     []string
		hasError bool
	}{
		{
			name:     "NewCommand_NoArgs",
			cmdFunc:  addNewCmd,
			args:     []string{},
			hasError: true,
		},
		{
			name:     "NewCommand_WithArgs",
			cmdFunc:  addNewCmd,
			args:     []string{"repo1"},
			hasError: false,
		},
		{
			name:     "EditCommand_NoArgs",
			cmdFunc:  addEditCmd,
			args:     []string{},
			hasError: true,
		},
		{
			name:     "EditCommand_WithArgs",
			cmdFunc:  addEditCmd,
			args:     []string{"repo1"},
			hasError: false,
		},
		{
			name:     "MergeCommand_NoArgs",
			cmdFunc:  addMergeCmd,
			args:     []string{},
			hasError: true,
		},
		{
			name:     "MergeCommand_WithArgs",
			cmdFunc:  addMergeCmd,
			args:     []string{"repo1"},
			hasError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.cmdFunc()

			err := cmd.Args(cmd, test.args)

			if test.hasError && err == nil {
				t.Error("Expected error but got none")
			}

			if !test.hasError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestPRRootCommand(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	// Test that all expected subcommands are registered
	expectedSubCommands := []string{"new", "edit", "merge", "get"}

	foundCommands := make(map[string]bool)
	for _, subCmd := range cmd.Commands() {
		foundCommands[subCmd.Name()] = true
	}

	for _, expectedCmd := range expectedSubCommands {
		if !foundCommands[expectedCmd] {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestLookupReviewersWithSCMRepositories(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Configure reviewers for repositories from fake provider
	viper.Set(config.DefaultReviewers, map[string][]string{
		"repo-1": {"alice", "bob"},
		"repo-2": {"charlie", "diana"},
		"repo-3": {"eve", "frank"},
	})

	t.Run("LookupDefaultReviewers", func(t *testing.T) {
		reviewers := lookupReviewers(ctx, "repo-1")
		expected := map[string]bool{"alice": true, "bob": true}

		if len(reviewers) != len(expected) {
			t.Errorf("Expected %d reviewers, got %d: %v", len(expected), len(reviewers), reviewers)
		}

		for _, reviewer := range reviewers {
			if !expected[reviewer] {
				t.Errorf("Unexpected reviewer %s in results %v", reviewer, reviewers)
			}
		}
	})

	t.Run("LookupGlobalReviewers", func(t *testing.T) {
		// CLI-provided reviewers take precedence
		viper.Set(config.PrReviewers, []string{"global1", "global2"})

		reviewers := lookupReviewers(ctx, "repo-1")
		expected := []string{"global1", "global2"}

		if len(reviewers) != len(expected) {
			t.Errorf("Expected %d global reviewers, got %d", len(expected), len(reviewers))
		}

		for i, reviewer := range reviewers {
			if reviewer != expected[i] {
				t.Errorf("Expected global reviewer %s at position %d, got %s", expected[i], i, reviewer)
			}
		}
	})

	t.Run("LookupReviewersForUnknownRepo", func(t *testing.T) {
		// Clear global reviewers
		viper.Set(config.PrReviewers, []string{})

		reviewers := lookupReviewers(ctx, "unknown-repo")
		if len(reviewers) != 0 {
			t.Errorf("Expected no reviewers for unknown repo, got %d", len(reviewers))
		}
	})
}
