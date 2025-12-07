package labels

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/catalog"
)

// Cmd configures the labels command
func Cmd() *cobra.Command {
	labelsCmd := &cobra.Command{
		Use:     "labels <repository|label> ...",
		Aliases: []string{"label"},
		Short:   "Inspect repository labels and test filters",
		//Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Import command(s) from the CLI flag
			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				return err
			}

			if len(args) > 0 {
				PrintSet(cmd.Context(), verbose, args...)
			} else {
				fmt.Println("Available labels:")
				PrintLabels(cmd.Context())
			}

			return nil
		},
	}

	labelsCmd.Flags().BoolP("verbose", "v", false, "expand labels referenced in the given filter")

	return labelsCmd
}
