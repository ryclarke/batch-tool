package call

import (
	"fmt"

	"github.com/spf13/cobra"
)

// OutputHandler represents a function for processing command output.
type OutputHandler func(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error)

// OrderedOutput is the default OutputHandler that batches and prints output from each repository's channels in sequence.
func OrderedOutput(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error) {
	for i, repo := range repos {
		// print header with repository name
		fmt.Fprintf(cmd.OutOrStdout(), "\n------ %s ------\n", repo)

		// print all output for this repo to Stdout
		for msg := range output[i] {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}

		// print any errors for this repo to Stderr
		for err := range errs[i] {
			fmt.Fprintln(cmd.ErrOrStderr(), "ERROR: ", err)
		}
	}
}
