package output

import (
	"context"
	"fmt"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/cobra"
)

const (
	Native    = "native"
	Bubbletea = "bubbletea"
)

var AvailableHandlers = []string{Native, Bubbletea}

// Handler represents a function for processing streaming command output.
type Handler func(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error)

// GetHandler returns an output Handler based on the configuration.
func GetHandler(ctx context.Context) Handler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Bubbletea:
		return BubbleteaHandler
	case Native:
		return NativeHandler
	default:
		// Fallback to native handler if unrecognized (or unset)
		return NativeHandler
	}
}

// NativeHandler is a simple output Handler that batches and prints output from each repository's channels in sequence.
// It is straightforward and compatible with all terminal environments, but lacks interactivity and modern UI features.
func NativeHandler(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error) {
	for i, repo := range repos {
		// print header with repository name
		fmt.Fprintf(cmd.OutOrStdout(), "\n------ %s ------", repo)

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
