package output

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
)

const (
	// TUI is the terminal UI output style
	TUI = "tui"
	// Native is the native output style
	Native = "native"
)

// AvailableStyles lists all supported output styles
var AvailableStyles = []string{TUI, Native}

// Handler represents a function for processing streaming command output.
type Handler func(cmd *cobra.Command, channels []Channel)

// GetHandler returns an output Handler based on the configuration.
func GetHandler(ctx context.Context) Handler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Native:
		return NativeHandler
	default:
		// Use more advanced TUI handler by default
		return TUIHandler
	}
}

// LabelHandler represents a function for displaying labels.
type LabelHandler func(cmd *cobra.Command, verbose bool, filters ...string)

// GetLabelHandler returns a LabelHandler based on the configuration.
func GetLabelHandler(ctx context.Context) LabelHandler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Native:
		return NativeLabels
	default:
		// Use more advanced TUI handler by default
		return TUILabels
	}
}

// CatalogHandler represents a function for displaying the repository catalog.
type CatalogHandler func(cmd *cobra.Command)

// GetCatalogHandler returns a CatalogHandler based on the configuration.
func GetCatalogHandler(ctx context.Context) CatalogHandler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Native:
		return NativeCatalog
	default:
		// Use more advanced TUI handler by default
		return TUICatalog
	}
}
