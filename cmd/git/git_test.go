package git

import (
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestGitCmd(t *testing.T) {
	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "git <command> [flags] <repository>..." {
		t.Errorf("Expected Use to be 'git <command> [flags] <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestGitCmdSubcommands(t *testing.T) {
	cmd := Cmd()

	subcommands := cmd.Commands()
	expectedCommands := []string{"status", "branch", "commit", "diff", "push", "update"}

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

func TestValidateBranchSourceBranchMatch(t *testing.T) {
	ctx := loadFixture(t)

	// Set up a temporary git repo for testing
	reposPath := testhelper.SetupRepos(t, []string{"test-repo"})

	// Update context to use the temp repos path
	viper := config.Viper(ctx)
	viper.Set(config.GitDirectory, reposPath)
	viper.Set(config.GitHost, "example.com")
	viper.Set(config.GitProject, "test-project")

	ch := output.NewChannel(ctx, "test-repo", nil, nil)
	err := ValidateBranch()(ctx, ch)
	ch.Close()

	if err == nil {
		t.Error("Expected error when current branch matches base branch")
	} else if !strings.Contains(err.Error(), "base branch") {
		t.Errorf("Error should mention base branch, got: %v", err)
	}
}
