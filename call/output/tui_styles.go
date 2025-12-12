package output

import (
	"context"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

// Color constants - Dracula theme
const (
	colorBackground  = "#282a36"
	colorCurrentLine = "#44475a"
	colorForeground  = "#f8f8f2"
	colorComment     = "#6272a4"
	colorCyan        = "#8be9fd"
	colorGreen       = "#50fa7b"
	colorGreenDark   = "#2d7a45" // Darker green for progress bar
	colorOrange      = "#ffb86c"
	colorPink        = "#ff79c6"
	colorPurple      = "#bd93f9"
	colorRed         = "#ff5555"
	colorRedDark     = "#8b2e2e" // Darker red for progress bar
	colorYellow      = "#f1fa8c"
)

// Common string constants and section formatting
const (
	separatorLine = "  ─────────────────────────────────────"

	summaryText  = "%d repositories | Elapsed: %s"
	progressText = "Progress: %d/" + summaryText

	noReposText = "No repositories matched by provided filter, nothing to do."
	footerText  = "↑/↓: scroll | PgUp/PgDn/Home/End: paging | also supports Vim keybinds"
	footerDone  = "✓ All done! " + footerText + " | p: print output | Enter/Esc or q: quit"
	tuiFailText = "Error running TUI: %v\nUsing fallback output handler...\n"

	repoWaitingFormat = "⏸ %s"
	repoActiveFormat  = "▶ %s"
	repoSuccessFormat = "✓ %s"
	repoErrorFormat   = "✗ %s"

	emptyLabelText  = "(empty label)"
	labelNameFormat = "# %s"
)

// -- Common style constructors

// creates a function that wraps styles to a specified width
func wrapStyleFunc(width int) func(style ...lipgloss.Style) lipgloss.Style {
	return func(styles ...lipgloss.Style) lipgloss.Style {
		var style lipgloss.Style
		if len(styles) > 0 {
			style = styles[0]
		} else {
			style = lipgloss.NewStyle()
		}

		return style.Width(width - 4).MarginLeft(2)
	}
}

// creates a common style for wrapping colored output text to a specified width
func wrapColor(textColor string, width int) lipgloss.Style {
	return wrapStyleFunc(width)(color(textColor))
}

// create a common style with the given foreground color
func color(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

// outputStyles contains styles for the main TUI output handler
type outputStyles struct {
	wrap func(style ...lipgloss.Style) lipgloss.Style

	repoActive  lipgloss.Style
	repoWaiting lipgloss.Style
	repoSuccess lipgloss.Style
	repoError   lipgloss.Style

	separator lipgloss.Style
	status    lipgloss.Style
	output    lipgloss.Style
	outputErr lipgloss.Style

	progress              lipgloss.Style
	progressBarIncomplete lipgloss.Style
	progressBarComplete   lipgloss.Style
	progressBarError      lipgloss.Style
}

func newOutputStyles(width int) outputStyles {
	return outputStyles{
		wrap: wrapStyleFunc(width),

		repoActive:  color(colorCyan).Bold(true),
		repoWaiting: color(colorComment).Bold(true),
		repoSuccess: color(colorGreen).Bold(true),
		repoError:   color(colorRed).Bold(true),

		separator: color(colorCurrentLine),
		status:    color(colorPurple).Italic(true),
		output:    wrapColor(colorForeground, width),
		outputErr: wrapColor(colorRed, width),

		progress:              color(colorCyan),
		progressBarIncomplete: color(colorCurrentLine).Background(lipgloss.Color(colorBackground)),
		progressBarComplete:   color(colorGreenDark).Background(lipgloss.Color(colorGreenDark)),
		progressBarError:      color(colorRedDark).Background(lipgloss.Color(colorRedDark)),
	}
}

// labelStyles contains styles for the labels display
type labelStyles struct {
	wrap func(style ...lipgloss.Style) lipgloss.Style

	repo     lipgloss.Style
	unwanted lipgloss.Style
	count    lipgloss.Style
	forced   lipgloss.Style
	excluded lipgloss.Style
	normal   lipgloss.Style
	symbol   lipgloss.Style

	separator lipgloss.Style
	section   lipgloss.Style
	title     lipgloss.Style
	help      lipgloss.Style
}

func newLabelStyles(width int) labelStyles {
	return labelStyles{
		wrap: wrapStyleFunc(width),

		repo:     color(colorForeground),
		unwanted: color(colorComment),
		count:    color(colorComment),
		forced:   color(colorGreen).Bold(true),
		excluded: color(colorOrange).Bold(true),
		normal:   color(colorPurple).Bold(true),
		symbol:   color(colorPurple),

		separator: color(colorCurrentLine),
		section:   color(colorCyan).Bold(true),
		title:     color(colorCyan).Bold(true),
		help:      color(colorComment),
	}
}

// catalogStyles contains styles for the catalog display
type catalogStyles struct {
	wrap func(style ...lipgloss.Style) lipgloss.Style

	title       lipgloss.Style
	repoName    lipgloss.Style
	description lipgloss.Style

	metaLabel   lipgloss.Style
	metaValue   lipgloss.Style
	publicRepo  lipgloss.Style
	privateRepo lipgloss.Style
	label       lipgloss.Style
	separator   lipgloss.Style
	help        lipgloss.Style
}

func newCatalogStyles(width int) catalogStyles {
	return catalogStyles{
		wrap: wrapStyleFunc(width),

		title:       color(colorCyan).Bold(true),
		repoName:    color(colorGreen).Bold(true),
		description: wrapColor(colorForeground, width),

		metaLabel:   color(colorComment),
		metaValue:   color(colorPurple),
		publicRepo:  color(colorPink),
		privateRepo: color(colorPink),
		label:       color(colorYellow),
		separator:   color(colorCurrentLine),
		help:        color(colorComment),
	}
}

// handleKeyPress processes keyboard input for viewport navigation.
// Returns true if the key was handled, false otherwise.
func handleKeyPress(vp *viewport.Model, key string) bool {
	switch key {
	// single line scrolling
	case "down", "j":
		vp.ScrollDown(1)
		return true
	case "up", "k":
		vp.ScrollUp(1)
		return true

	// half page scrolling
	case "shift+down", "J", "shift+j", "ctrl+d":
		vp.HalfPageDown()
		return true
	case "shift+up", "K", "shift+k", "ctrl+u":
		vp.HalfPageUp()
		return true

	// full page scrolling
	case "pgdown", "ctrl+f":
		vp.PageDown()
		return true
	case "pgup", "ctrl+b":
		vp.PageUp()
		return true

	// jump to top/bottom
	case "home", "g":
		vp.GotoTop()
		return true
	case "end", "G", "shift+g":
		vp.GotoBottom()
		return true

	default:
		return false
	}
}

// renderProgressBar creates a visual progress bar with error indication
func renderProgressBar(styles outputStyles, completed, errors, total, width int) string {
	if width < 10 {
		width = 40 // minimum width
	}

	if total == 0 {
		return styles.progressBarIncomplete.Render(strings.Repeat(" ", width))
	}

	// Calculate widths for success (green) and error (red) portions
	successCount := completed - errors
	successPercent := float64(successCount) / float64(total)
	errorPercent := float64(errors) / float64(total)
	successWidth := int(float64(width) * successPercent)
	errorWidth := int(float64(width) * errorPercent)
	emptyWidth := width - successWidth - errorWidth

	var bar strings.Builder

	// Green portion for successful completions
	if successWidth > 0 {
		bar.WriteString(styles.progressBarComplete.Render(strings.Repeat("█", successWidth)))
	}

	// Red portion for errors
	if errorWidth > 0 {
		bar.WriteString(styles.progressBarError.Render(strings.Repeat("█", errorWidth)))
	}

	// Gray portion for incomplete
	if emptyWidth > 0 {
		bar.WriteString(styles.progressBarIncomplete.Render(strings.Repeat("░", emptyWidth)))
	}

	return bar.String()
}

// Helper to get unwanted repos set
func getUnwantedRepos(ctx context.Context) mapset.Set[string] {
	unwantedRepos := mapset.NewSet[string]()

	for _, unwanted := range config.Viper(ctx).GetStringSlice(config.UnwantedLabels) {
		if set, ok := catalog.Labels[unwanted]; ok {
			unwantedRepos = unwantedRepos.Union(set)
		}
	}

	return unwantedRepos
}

// Helper to check if a label is unwanted
func isLabelUnwanted(ctx context.Context, labelName string) bool {
	viper := config.Viper(ctx)
	unwantedLabels := viper.GetStringSlice(config.UnwantedLabels)

	return slices.Contains(unwantedLabels, labelName)
}
