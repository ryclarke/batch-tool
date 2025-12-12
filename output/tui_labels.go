package output

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// TUILabels displays labels using a TUI for an interactive experience.
// When no filters are provided, it displays all available labels with their repositories.
// When filters are provided, it displays a concise set-theory representation with matched repos.
func TUILabels(cmd *cobra.Command, verbose bool, filters ...string) {
	ctx := cmd.Context()
	var m tea.Model

	if len(filters) > 0 {
		m = newLabelsFilterModel(ctx, verbose, filters)
	} else {
		m = newLabelsListModel(ctx, verbose)
	}

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), tuiFailText, err)
		// Fallback to native output handling
		NativeLabels(cmd, verbose, filters...)
		return
	}

	// If the user requested to persist output (via 'p' key or --print flag), print it when exiting
	switch m := finalModel.(type) {
	case labelsFilterModel:
		if m.printOutput || !m.waitOnExit {
			m.printFullOutput(cmd)
		}
	case labelsListModel:
		if m.printOutput || !m.waitOnExit {
			m.printFullOutput(cmd)
		}
	}
}

// labelsListModel displays all available labels and their repositories
type labelsListModel struct {
	ctx      context.Context
	labels   []labelWithRepos
	viewport viewport.Model
	ready    bool
	width    int
	height   int
	verbose  bool

	printOutput bool
	waitOnExit  bool
}

type labelWithRepos struct {
	name       string
	repos      []string
	empty      bool
	isUnwanted bool
}

func newLabelsListModel(ctx context.Context, verbose bool) labelsListModel {
	viper := config.Viper(ctx)
	labels := make([]labelWithRepos, 0)

	labelNames := make([]string, 0, len(catalog.Labels))
	for label := range catalog.Labels {
		if label == viper.GetString(config.SuperSetLabel) {
			continue
		}
		labelNames = append(labelNames, label)
	}
	sort.Strings(labelNames)

	for _, label := range labelNames {
		isUnwanted := isLabelUnwanted(ctx, label)

		// Skip unwanted labels unless verbose mode is enabled
		if !verbose && isUnwanted {
			continue
		}

		if set, ok := catalog.Labels[label]; ok && set.Cardinality() > 0 {
			repos := set.ToSlice()
			if viper.GetBool(config.SortRepos) {
				sort.Strings(repos)
			}
			labels = append(labels, labelWithRepos{name: label, repos: repos, isUnwanted: isUnwanted})
		} else {
			labels = append(labels, labelWithRepos{name: label, empty: true, isUnwanted: isUnwanted})
		}
	}

	return labelsListModel{
		ctx:     ctx,
		labels:  labels,
		verbose: verbose,

		printOutput: viper.GetBool(config.PrintResults),
		waitOnExit:  viper.GetBool(config.WaitOnExit),
	}
}

func (m labelsListModel) Init() tea.Cmd {
	return nil
}

func (m labelsListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			headerHeight := 3
			footerHeight := 2
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
			// Auto-quit if wait flag is false
			if !m.waitOnExit {
				return m, tea.Quit
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 5
		}

		m.viewport.SetContent(m.buildContent())
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			m.printOutput = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			return m, tea.Quit

		default:
			// Use shared viewport navigation handler
			if handleKeyPress(&m.viewport, msg.String()) {
				return m, nil
			}

			// Let viewport handle all other keys for scrolling
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m labelsListModel) buildContent() string {
	unwantedRepos := getUnwantedRepos(m.ctx)
	styles := newLabelStyles(m.width)
	var b strings.Builder

	for i, label := range m.labels {
		m.buildLabelContent(&b, styles, label, unwantedRepos)

		// Add separator between labels (except for the last one)
		if i < len(m.labels)-1 {
			b.WriteString(styles.separator.Render(separatorLine))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m labelsListModel) buildLabelContent(b *strings.Builder, styles labelStyles, label labelWithRepos, unwantedRepos mapset.Set[string]) {
	// Use brown color for unwanted labels
	if label.isUnwanted {
		b.WriteString(styles.excluded.Render(fmt.Sprintf(labelNameFormat, label.name)))
	} else {
		b.WriteString(styles.normal.Render(fmt.Sprintf(labelNameFormat, label.name)))
	}
	b.WriteString(" ")

	if label.empty {
		b.WriteString(styles.count.Render(emptyLabelText))
		b.WriteString("\n")
	} else {
		// Calculate wanted vs total repos
		totalRepos := len(label.repos)
		wantedRepos := 0
		for _, repo := range label.repos {
			if !unwantedRepos.Contains(repo) {
				wantedRepos++
			}
		}

		// For unwanted labels, always show just total count
		// For wanted labels, show "X / Y" format only if some repos are unwanted, otherwise just "Y"
		if label.isUnwanted || wantedRepos == totalRepos {
			b.WriteString(styles.count.Render(fmt.Sprintf("(%d)", totalRepos)))
		} else {
			b.WriteString(styles.count.Render(fmt.Sprintf("(%d / %d)", wantedRepos, totalRepos)))
		}
		b.WriteString("\n")

		// Show the list of repositories matched by this label
		// Color repos grey if they're matched by unwanted labels
		m.buildStyledRepos(b, label, styles, unwantedRepos)
	}
}

func (m labelsListModel) buildStyledRepos(b *strings.Builder, label labelWithRepos, styles labelStyles, unwantedRepos mapset.Set[string]) {
	// Style each repo individually based on whether it's unwanted
	styledRepos := make([]string, len(label.repos))
	for j, repo := range label.repos {
		if unwantedRepos.Contains(repo) || label.isUnwanted {
			styledRepos[j] = styles.unwanted.Render(repo)
		} else {
			styledRepos[j] = styles.repo.Render(repo)
		}
	}

	// Join styled repos without additional styling
	b.WriteString(styles.wrap(lipgloss.NewStyle()).Render(strings.Join(styledRepos, ", ")))
	b.WriteString("\n")
}

func (m labelsListModel) View() string {
	if !m.ready {
		return "Loading labels..."
	}

	styles := newLabelStyles(m.width)
	var b strings.Builder

	b.WriteString(styles.title.Render("Available Labels:"))
	b.WriteString("\n\n")

	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	b.WriteString(footerDone)

	return b.String()
}

// labelsFilterModel displays filtered labels with set-theory representation
type labelsFilterModel struct {
	ctx        context.Context
	verbose    bool
	filters    []string
	labelGroup catalog.LabelGroup
	repos      []string
	labels     struct {
		forced   []labelWithRepos
		included []labelWithRepos
		excluded []labelWithRepos
	}
	viewport viewport.Model
	ready    bool
	width    int
	height   int

	printOutput bool
	waitOnExit  bool
}

func newLabelsFilterModel(ctx context.Context, verbose bool, filters []string) labelsFilterModel {
	viper := config.Viper(ctx)

	labelGroup, repos := catalog.ParseLabels(ctx, filters...)

	m := labelsFilterModel{
		ctx:        ctx,
		verbose:    verbose,
		filters:    filters,
		labelGroup: labelGroup,
		repos:      repos,

		printOutput: viper.GetBool(config.PrintResults),
		waitOnExit:  viper.GetBool(config.WaitOnExit),
	}

	// If verbose, build label details from parsed label group
	if verbose {
		m.verboseInit(ctx, labelGroup)
	}

	return m
}

// verboseInit populates the labelWithRepos slices for each label category.
func (m *labelsFilterModel) verboseInit(ctx context.Context, labelGroup catalog.LabelGroup) {
	forced, included, excluded := labelGroup.ToSlices()

	m.labels.forced = buildLabelWithRepos(ctx, forced)
	m.labels.included = buildLabelWithRepos(ctx, included)
	m.labels.excluded = buildLabelWithRepos(ctx, excluded)
}

// buildLabelWithRepos converts label names into labelWithRepos structs with their repositories.
func buildLabelWithRepos(ctx context.Context, labelNames []string) []labelWithRepos {
	if len(labelNames) == 0 {
		return nil
	}

	viper := config.Viper(ctx)
	labels := make([]labelWithRepos, 0, len(labelNames))

	for _, label := range labelNames {
		if set, ok := catalog.Labels[utils.CleanFilter(ctx, label)]; ok && set.Cardinality() > 0 {
			repos := set.ToSlice()
			if viper.GetBool(config.SortRepos) {
				sort.Strings(repos)
			}
			labels = append(labels, labelWithRepos{name: label, repos: repos})
		} else {
			labels = append(labels, labelWithRepos{name: label, empty: true})
		}
	}

	return labels
}

func (m labelsFilterModel) Init() tea.Cmd {
	return nil
}

func (m labelsFilterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			headerHeight := 5
			footerHeight := 2
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
			// Auto-quit if wait flag is false
			if !m.waitOnExit {
				return m, tea.Quit
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 7
		}

		m.viewport.SetContent(m.buildContent(m.ctx))
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			m.printOutput = true
			return m, tea.Quit
		case "enter", "esc", "q", "ctrl+c":
			return m, tea.Quit

		default:
			// Use shared viewport navigation handler
			if handleKeyPress(&m.viewport, msg.String()) {
				return m, nil
			}

			// Let viewport handle all other keys for scrolling
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m labelsFilterModel) buildSetString() string {
	styles := newLabelStyles(m.width)

	const (
		union = "∪" // U+222A
		minus = "∖" // U+2216
	)

	var b strings.Builder

	// Build colored set notation
	if m.labelGroup.Forced.Cardinality() > 0 {
		// Forced labels (included regardless of exclusions)
		b.WriteString(styles.symbol.Render("("))
		b.WriteString(styles.forced.Render(m.labelGroup.Forced.String()))
		b.WriteString(styles.symbol.Render(")"))

		if m.labelGroup.Included.Cardinality() == 0 {
			// if labels are only forced or excluded, then the exclusions aren't relevant
			return b.String()
		}

		b.WriteString(styles.symbol.Render(" " + union + " "))
		if m.labelGroup.Excluded.Cardinality() > 0 {
			b.WriteString(styles.symbol.Render("( "))
		}
	}

	// Included labels
	b.WriteString(styles.symbol.Render("("))
	b.WriteString(styles.normal.Render(m.labelGroup.Included.String()))
	b.WriteString(styles.symbol.Render(")"))

	if m.labelGroup.Excluded.Cardinality() > 0 {
		// Excluded labels (not included unless forced)
		b.WriteString(styles.symbol.Render(" " + minus + " "))
		b.WriteString(styles.symbol.Render("("))
		b.WriteString(styles.excluded.Render(m.labelGroup.Excluded.String()))
		b.WriteString(styles.symbol.Render(")"))

		if m.labelGroup.Forced.Cardinality() > 0 {
			b.WriteString(styles.symbol.Render(" )"))
		}
	}

	return styles.wrap().Render(b.String())
}

func (m labelsFilterModel) buildContent(ctx context.Context) string {
	var b strings.Builder
	styles := newLabelStyles(m.width)

	// Matched repositories summary
	b.WriteString("This matches ")
	switch n := len(m.repos); n {
	case 0:
		b.WriteString(styles.forced.Foreground(lipgloss.Color(colorYellow)).Render("no known repositories"))
		b.WriteString("\n")
	case 1:
		b.WriteString(styles.forced.Render("1 repository"))
		b.WriteString(": ")
		b.WriteString(styles.wrap(styles.repo).Render(m.repos[0]))
		b.WriteString("\n")
	default:
		b.WriteString(styles.forced.Render(fmt.Sprintf("%d repositories", n)))
		b.WriteString(":\n")
		b.WriteString(styles.wrap(styles.repo).Render(strings.Join(m.repos, ", ")))
		b.WriteString("\n")
	}

	// Verbose details
	if m.verbose {
		if len(m.labels.forced) > 0 {
			m.buildVerboseContent(ctx, m.labels.forced, &b, styles, styles.forced, styles.wrap(styles.repo), "Forced")
		}

		if len(m.labels.included) > 0 {
			m.buildVerboseContent(ctx, m.labels.included, &b, styles, styles.normal, styles.wrap(styles.repo), "Included")
		}

		if len(m.labels.excluded) > 0 {
			m.buildVerboseContent(ctx, m.labels.excluded, &b, styles, styles.excluded, styles.wrap(styles.unwanted), "Excluded")
		}
	}

	return b.String()
}

func (m labelsFilterModel) buildVerboseContent(ctx context.Context, set []labelWithRepos, b *strings.Builder, styles labelStyles, labelStyle, repoStyle lipgloss.Style, kind string) {
	b.WriteString("\n")
	b.WriteString(styles.separator.Render(separatorLine))
	b.WriteString("\n")
	b.WriteString(styles.section.Render(kind + " labels:"))
	b.WriteString("\n")

	for _, label := range set {
		// only print verbose details for labels, not individual repositories
		if !strings.Contains(label.name, config.Viper(ctx).GetString(config.TokenLabel)) {
			continue
		}

		b.WriteString(labelStyle.Render(fmt.Sprintf(labelNameFormat+"\n", utils.CleanFilter(ctx, label.name))))
		if label.empty {
			b.WriteString(repoStyle.Render(styles.count.Render(emptyLabelText)))
		} else {
			b.WriteString(repoStyle.Render(strings.Join(label.repos, ", ")))
		}
		b.WriteString("\n")
	}
}

func (m labelsFilterModel) View() string {
	if !m.ready {
		return "Processing filters..."
	}

	var b strings.Builder
	styles := newLabelStyles(m.width)

	// Title and set representation
	b.WriteString(styles.title.Render("Selected Set:"))
	b.WriteString("\n")
	b.WriteString(m.buildSetString())
	b.WriteString("\n\n")

	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	b.WriteString(styles.help.Render(footerDone))

	return b.String()
}

// printFullOutput prints the complete labels list output to stdout, reusing the buildContent logic
func (m labelsListModel) printFullOutput(cmd *cobra.Command) {
	styles := newLabelStyles(m.width)
	out := cmd.OutOrStdout()

	// Title
	fmt.Fprintln(out, styles.title.Render("Available Labels:"))
	fmt.Fprintln(out)

	// Print content (reuses buildContent)
	fmt.Fprint(out, m.buildContent())
}

// printFullOutput prints the complete filtered labels output to stdout, reusing the buildContent logic
func (m labelsFilterModel) printFullOutput(cmd *cobra.Command) {
	styles := newLabelStyles(m.width)
	out := cmd.OutOrStdout()

	// Title and set representation
	fmt.Fprintln(out, styles.title.Render("Selected Set:"))
	fmt.Fprintln(out, m.buildSetString())
	fmt.Fprintln(out)

	// Print content (reuses buildContent)
	fmt.Fprint(out, m.buildContent(m.ctx))
}
