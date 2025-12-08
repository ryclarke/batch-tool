package pr

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func loadFixture(t *testing.T) context.Context {
	return config.LoadFixture(t, "../../config")
}

func TestPrCmd(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "pr [cmd] <repository> ..." {
		t.Errorf("Expected Use to be 'pr [cmd] <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestPrCmdSubcommands(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	subcommands := cmd.Commands()
	expectedCommands := []string{"new", "edit", "merge"}

	if len(subcommands) < len(expectedCommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedCommands), len(subcommands))
	}

	// Check that expected subcommands exist
	commandNames := make(map[string]bool)
	for _, subcmd := range subcommands {
		commandNames[subcmd.Name()] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !commandNames[expectedCmd] {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestPrCmdFlags(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	// Test persistent flags
	titleFlag := cmd.PersistentFlags().Lookup("title")
	if titleFlag == nil {
		t.Error("title flag not found")

		return
	}

	if titleFlag.Shorthand != "t" {
		t.Errorf("Expected title flag shorthand to be 't', got %s", titleFlag.Shorthand)
	}

	descFlag := cmd.PersistentFlags().Lookup("description")
	if descFlag == nil {
		t.Error("description flag not found")

		return
	}

	if descFlag.Shorthand != "d" {
		t.Errorf("Expected description flag shorthand to be 'd', got %s", descFlag.Shorthand)
	}

	reviewerFlag := cmd.PersistentFlags().Lookup("reviewer")
	if reviewerFlag == nil {
		t.Error("reviewer flag not found")

		return
	}

	if reviewerFlag.Shorthand != "r" {
		t.Errorf("Expected reviewer flag shorthand to be 'r', got %s", reviewerFlag.Shorthand)
	}
}

func TestPrCmdArgs(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	// Test that command requires minimum arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test that command accepts arguments
	err = cmd.Args(cmd, []string{"repo1"})
	if err != nil {
		t.Errorf("Expected no error with valid arguments, got %v", err)
	}
}

func TestPrCmdPersistentPreRunE(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	cmd := Cmd()
	cmd.SetContext(ctx)

	// Test PersistentPreRunE function exists
	if cmd.PersistentPreRunE == nil {
		t.Error("Expected PersistentPreRunE function to be set")

		return
	}

	// Test without auth token (should require auth token)
	viper.Set(config.AuthToken, "")
	err := cmd.PersistentPreRunE(cmd, []string{})
	if err == nil {
		t.Error("Expected error when auth token is not set")
	}
}

func TestLookupReviewers(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Test with command-line reviewers
	viper.Set(config.PrReviewers, []string{"reviewer1", "reviewer2"})
	reviewers := lookupReviewers(ctx, "test-repo")

	if len(reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(reviewers))
	}
	if reviewers[0] != "reviewer1" || reviewers[1] != "reviewer2" {
		t.Errorf("Expected [reviewer1, reviewer2], got %v", reviewers)
	}

	// Test with default reviewers for repository
	viper.Set(config.PrReviewers, []string{}) // Clear command-line reviewers
	defaultReviewers := map[string][]string{
		"test-repo":  {"default1", "default2"},
		"other-repo": {"other1"},
	}
	viper.Set(config.DefaultReviewers, defaultReviewers)

	reviewers = lookupReviewers(ctx, "test-repo")
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 default reviewers, got %d", len(reviewers))
	}
	if reviewers[0] != "default1" || reviewers[1] != "default2" {
		t.Errorf("Expected [default1, default2], got %v", reviewers)
	}

	// Test with non-existent repository
	reviewers = lookupReviewers(ctx, "nonexistent-repo")
	if len(reviewers) != 0 {
		t.Errorf("Expected 0 reviewers for nonexistent repo, got %d", len(reviewers))
	}
}

func TestAddNewCmd(t *testing.T) {
	cmd := addNewCmd()

	if cmd == nil {
		t.Fatal("addNewCmd() returned nil")
	}

	if cmd.Use != "new <repository> ..." {
		t.Errorf("Expected Use to be 'new <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestNewCmdFlags(t *testing.T) {
	cmd := addNewCmd()

	// Test all-reviewers flag
	allReviewersFlag := cmd.Flags().Lookup("all-reviewers")
	if allReviewersFlag == nil {
		t.Error("all-reviewers flag not found")

		return
	}

	if allReviewersFlag.Shorthand != "a" {
		t.Errorf("Expected all-reviewers flag shorthand to be 'a', got %s", allReviewersFlag.Shorthand)
	}
}

func TestNewCmdArgs(t *testing.T) {
	cmd := addNewCmd()

	// Test that command requires minimum arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test that command accepts arguments
	err = cmd.Args(cmd, []string{"repo1"})
	if err != nil {
		t.Errorf("Expected no error with valid arguments, got %v", err)
	}
}

func TestAddEditCmd(t *testing.T) {
	cmd := addEditCmd()

	if cmd == nil {
		t.Fatal("addEditCmd() returned nil")
	}

	if cmd.Use != "edit <repository> ..." {
		t.Errorf("Expected Use to be 'edit <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestEditCmdFlags(t *testing.T) {
	cmd := addEditCmd()

	// Test reset-reviewers flag
	resetReviewersFlag := cmd.Flags().Lookup("reset-reviewers")
	if resetReviewersFlag == nil {
		t.Error("reset-reviewers flag not found")
	}
}

func TestEditCmdArgs(t *testing.T) {
	cmd := addEditCmd()

	// Test that command requires minimum arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test that command accepts arguments
	err = cmd.Args(cmd, []string{"repo1"})
	if err != nil {
		t.Errorf("Expected no error with valid arguments, got %v", err)
	}
}

func TestAddMergeCmd(t *testing.T) {
	cmd := addMergeCmd()

	if cmd == nil {
		t.Fatal("addMergeCmd() returned nil")
	}

	if cmd.Use != "merge <repository> ..." {
		t.Errorf("Expected Use to be 'merge <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestMergeCmdArgs(t *testing.T) {
	cmd := addMergeCmd()

	// Test that command requires minimum arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test that command accepts arguments
	err = cmd.Args(cmd, []string{"repo1"})
	if err != nil {
		t.Errorf("Expected no error with valid arguments, got %v", err)
	}
}
