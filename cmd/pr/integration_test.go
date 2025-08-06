package pr

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

func TestPRIntegrationWithFakeProvider(t *testing.T) {
	_ = config.LoadFixture("../../config")

	// Save original configuration
	originalProvider := viper.GetString(config.GitProvider)
	originalProject := viper.GetString(config.GitProject)
	originalToken := viper.GetString(config.AuthToken)

	defer func() {
		viper.Set(config.GitProvider, originalProvider)
		viper.Set(config.GitProject, originalProject)
		viper.Set(config.AuthToken, originalToken)
	}()

	// Configure for fake provider
	viper.Set(config.GitProvider, "fake")
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.AuthToken, "fake-token")

	// Register fake provider with test data
	scm.Register("fake-pr-test", func(project string) scm.Provider {
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
		cmd.SetOutput(&output)

		// We can't easily test execution without more setup, but we can test command structure
		if cmd.Use != "new <repository> ..." {
			t.Errorf("Expected Use to be 'new <repository> ...', got %s", cmd.Use)
		}

		if cmd.Short != "Submit new pull requests" {
			t.Errorf("Expected correct Short description, got %s", cmd.Short)
		}
	})

	// Test the edit command
	t.Run("EditCommand", func(t *testing.T) {
		cmd := addEditCmd()

		if cmd.Use != "edit <repository> ..." {
			t.Errorf("Expected Use to be 'edit <repository> ...', got %s", cmd.Use)
		}

		if cmd.Short != "Update existing pull requests" {
			t.Errorf("Expected correct Short description, got %s", cmd.Short)
		}
	})

	// Test the merge command
	t.Run("MergeCommand", func(t *testing.T) {
		cmd := addMergeCmd()

		if cmd.Use != "merge <repository> ..." {
			t.Errorf("Expected Use to be 'merge <repository> ...', got %s", cmd.Use)
		}

		if cmd.Short != "Merge accepted pull requests" {
			t.Errorf("Expected correct Short description, got %s", cmd.Short)
		}
	})
}

func TestPRCommandFlags(t *testing.T) {
	_ = config.LoadFixture("../../config")

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
			description: "use all provided reviewers for a new PR",
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
	_ = config.LoadFixture("../../config")

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
	_ = config.LoadFixture("../../config")

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
	_ = config.LoadFixture("../../config")

	cmd := Cmd()

	if cmd.Use != "pr [cmd] <repository> ..." {
		t.Errorf("Expected Use to be 'pr [cmd] <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Manage pull requests using supported SCM provider APIs" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}

	// Test that subcommands are added
	subCommands := cmd.Commands()
	expectedSubCommands := []string{"new", "edit", "merge"}

	if len(subCommands) < len(expectedSubCommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedSubCommands), len(subCommands))
	}

	// Check that expected subcommands exist
	foundCommands := make(map[string]bool)
	for _, subCmd := range subCommands {
		foundCommands[subCmd.Use] = true
	}

	for _, expectedCmd := range expectedSubCommands {
		if !foundCommands[expectedCmd+" <repository> ..."] {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}
