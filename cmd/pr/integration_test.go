package pr

import (
	"bytes"
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

	// Test the new command
	t.Run("NewCommand", func(t *testing.T) {
		cmd := addNewCmd()

		// Set up command with fake arguments
		cmd.SetArgs([]string{"repo-1"})

		// Capture output
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)

		// We can't easily test execution without more setup, but we can test command structure
		if cmd.Use != "new [--draft] [-t <title>] [-d <description>] [-r <reviewer>]... [-a] [-b <base-branch>] <repository>..." {
			t.Errorf("Expected Use to be 'new [--draft] [-t <title>] [-d <description>] [-r <reviewer>]... [-a] [-b <base-branch>] <repository>...', got %s", cmd.Use)
		}

		if cmd.Short == "" {
			t.Error("Expected Short description to be set")
		}
	})

	// Test the edit command
	t.Run("EditCommand", func(t *testing.T) {
		cmd := addEditCmd()

		if cmd.Use != "edit [--draft|--no-draft] [-t <title>] [-d <description>] [-r <reviewer>]... [--reset-reviewers] <repository>..." {
			t.Errorf("Expected Use to be 'edit [--draft|--no-draft] [-t <title>] [-d <description>] [-r <reviewer>]... [--reset-reviewers] <repository>...', got %s", cmd.Use)
		}

		if cmd.Short == "" {
			t.Error("Expected Short description to be set")
		}
	})

	// Test the merge command
	t.Run("MergeCommand", func(t *testing.T) {
		cmd := addMergeCmd()

		if cmd.Use != "merge [-f] <repository>..." {
			t.Errorf("Expected Use to be 'merge [-f] <repository>...', got %s", cmd.Use)
		}

		if cmd.Short == "" {
			t.Error("Expected Short description to be set")
		}
	})
}

func TestPRCommandFlags(t *testing.T) {
	loadFixture(t)

	tests := []struct {
		name        string
		cmdFunc     func() *cobra.Command
		flagName    string
		shorthand   string
		description string
	}{
		{
			name:        "NewCommand_AllReviewers",
			cmdFunc:     addNewCmd,
			flagName:    "all-reviewers",
			shorthand:   "a",
			description: "use all provided reviewers for a new PR (default: only the first)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.cmdFunc()

			flag := cmd.Flags().Lookup(test.flagName)
			if flag == nil {
				t.Errorf("Flag %s not found", test.flagName)
				return
			}

			if flag.Shorthand != test.shorthand {
				t.Errorf("Expected shorthand %s, got %s", test.shorthand, flag.Shorthand)
			}

			if flag.Usage != test.description {
				t.Errorf("Expected usage %s, got %s", test.description, flag.Usage)
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

	if cmd.Use != "pr <repository>..." {
		t.Errorf("Expected Use to be 'pr <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	// Test that subcommands are added
	subCommands := cmd.Commands()
	expectedSubCommands := []string{"new", "edit", "merge", "get"}

	if len(subCommands) < len(expectedSubCommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedSubCommands), len(subCommands))
	}

	// Check that expected subcommands exist by checking the command name (not full Use string)
	foundCommands := make(map[string]bool)
	for _, subCmd := range subCommands {
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
		// Default reviewers are limited to first one unless allReviewers flag is set
		viper.Set(config.PrAllReviewers, false)
		reviewers := lookupReviewers(ctx, "repo-1")
		expected := []string{"alice"}

		if len(reviewers) != len(expected) {
			t.Errorf("Expected %d reviewers, got %d", len(expected), len(reviewers))
		}

		for i, reviewer := range reviewers {
			if reviewer != expected[i] {
				t.Errorf("Expected reviewer %s at position %d, got %s", expected[i], i, reviewer)
			}
		}

		// With allReviewers flag, should return all default reviewers
		viper.Set(config.PrAllReviewers, true)
		reviewers = lookupReviewers(ctx, "repo-1")
		expectedAll := []string{"alice", "bob"}

		if len(reviewers) != len(expectedAll) {
			t.Errorf("Expected %d reviewers with allReviewers flag, got %d", len(expectedAll), len(reviewers))
		}

		for i, reviewer := range reviewers {
			if reviewer != expectedAll[i] {
				t.Errorf("Expected reviewer %s at position %d, got %s", expectedAll[i], i, reviewer)
			}
		}
	})

	t.Run("LookupGlobalReviewers", func(t *testing.T) {
		// Set global reviewers (should override default)
		// Manually-provided reviewers always return all, regardless of allReviewers flag
		viper.Set(config.PrReviewers, []string{"global1", "global2"})
		viper.Set(config.PrAllReviewers, false) // Flag doesn't affect manually-provided reviewers

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
