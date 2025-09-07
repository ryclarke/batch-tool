package multichange

import (
	"github.com/spf13/cobra"
)

// Cmd returns the multichange subcommand
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multichange",
		Short: "Apply tested changes from one repository to other repositories",
		Long: `Apply tested changes from one repository to other repositories

This command allows you to apply changes that have been tested in a source repository
to other repositories. The workflow is:

1. Make changes in a source repository
2. Test and validate the changes work
3. Use this command to apply the same changes to other repositories

The command compares old/new file pairs from the source repository with files in
target repositories. If the old file matches the target repo file exactly,
it applies the new file changes.`,
	}

	cmd.AddCommand(
		extractCmd(),
		applyCmd(),
		NewRevertCommand(),
		cleanupCmd(),
	)

	return cmd
}
