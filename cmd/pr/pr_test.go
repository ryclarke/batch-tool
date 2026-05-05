package pr

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
)

func TestPrCmd(t *testing.T) {
	loadFixture(t)

	if Cmd() == nil {
		t.Fatal("Cmd() returned nil")
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

	// Common PR CRUD flags are local to new/edit commands, not root pr
	for _, name := range []string{"title", "description", "reviewer", "team-reviewer", "draft", "no-draft"} {
		if cmd.Flag(name) != nil {
			t.Errorf("Did not expect root pr command to expose %q flag", name)
		}
	}
}

func TestBuildCommonPRFlags(t *testing.T) {
	cmd := &cobra.Command{}
	buildCommonPRFlags(cmd)

	tests := []struct {
		name      string
		shorthand string
	}{
		{name: "title", shorthand: "t"},
		{name: "description", shorthand: "d"},
		{name: "reviewer", shorthand: "r"},
		{name: "team-reviewer", shorthand: "R"},
		{name: "draft", shorthand: ""},
		{name: "no-draft", shorthand: ""},
	}

	for _, tt := range tests {
		flag := cmd.Flag(tt.name)
		if flag == nil {
			t.Fatalf("Expected flag %q to exist", tt.name)
		}

		if flag.Shorthand != tt.shorthand {
			t.Errorf("Flag %q shorthand mismatch: want %q, got %q", tt.name, tt.shorthand, flag.Shorthand)
		}
	}
}

func TestParseCommonPRFlags(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	buildCommonPRFlags(cmd)

	if err := cmd.Flags().Set("title", "T"); err != nil {
		t.Fatalf("Failed to set title flag: %v", err)
	}

	if err := cmd.Flags().Set("description", "D"); err != nil {
		t.Fatalf("Failed to set description flag: %v", err)
	}

	if err := cmd.Flags().Set("reviewer", "alice,bob"); err != nil {
		t.Fatalf("Failed to set reviewer flag: %v", err)
	}

	if err := cmd.Flags().Set("team-reviewer", "platform"); err != nil {
		t.Fatalf("Failed to set team-reviewer flag: %v", err)
	}

	if err := cmd.PersistentFlags().Set("no-draft", "true"); err != nil {
		t.Fatalf("Failed to set no-draft flag: %v", err)
	}

	if err := parseCommonPRFlags(cmd); err != nil {
		t.Fatalf("parseCommonPRFlags failed: %v", err)
	}

	if got := viper.GetString(config.PrTitle); got != "T" {
		t.Errorf("Expected title T, got %q", got)
	}

	if got := viper.GetString(config.PrDescription); got != "D" {
		t.Errorf("Expected description D, got %q", got)
	}

	if got := viper.GetStringSlice(config.PrReviewers); len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Errorf("Expected reviewers [alice bob], got %v", got)
	}

	if got := viper.GetStringSlice(config.PrTeamReviewers); len(got) != 1 || got[0] != "platform" {
		t.Errorf("Expected team reviewers [platform], got %v", got)
	}

	if got := viper.GetBool(config.PrDraft); got {
		t.Errorf("Expected draft=false when no-draft is set, got true")
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
