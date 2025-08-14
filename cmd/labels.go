package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/catalog"
)

func addLabelsCmd() *cobra.Command {
	// labelsCmd represents the labels command
	labelsCmd := &cobra.Command{
		Use:     "labels <repository|label> ...",
		Aliases: []string{"label"},
		Short:   "Inspect repository labels and test filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Import command(s) from the CLI flag
			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				return err
			}

			if len(args) > 0 {
				catalog.PrintSet(verbose, args...)
			} else {
				fmt.Println("Available labels:")
				catalog.PrintLabels()
			}

			return nil
		},
	}

	labelsCmd.Flags().BoolP("verbose", "v", false, "expand labels referenced in the given filter")

	return labelsCmd
}
