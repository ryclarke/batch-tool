package output

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

// TUICatalog displays the repository catalog using a TUI for an interactive experience.
func TUICatalog(cmd *cobra.Command) {
	ctx := cmd.Context()
	m := newCatalogModel(ctx)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), tuiFailText, err)
		// Fallback to native output
		NativeCatalog(cmd)
		return
	}

	// If the user requested to persist output (via 'p' key or --print flag), print it
	if m, ok := finalModel.(catalogModel); ok && (m.printOutput || !m.waitOnExit) {
		m.printFullOutput(cmd)
	}
}

// catalogModel displays the repository catalog with metadata
type catalogModel struct {
	ctx      context.Context
	repos    []repoWithMetadata
	viewport viewport.Model
	ready    bool
	width    int
	height   int

	printOutput bool
	waitOnExit  bool
}

type repoWithMetadata struct {
	name          string
	description   string
	project       string
	defaultBranch string
	labels        []string
	isPublic      bool
}

func newCatalogModel(ctx context.Context) catalogModel {
	viper := config.Viper(ctx)
	repos := make([]repoWithMetadata, 0, len(catalog.Catalog))

	// Convert catalog to sorted slice
	repoNames := make([]string, 0, len(catalog.Catalog))
	for name := range catalog.Catalog {
		repoNames = append(repoNames, name)
	}
	sort.Strings(repoNames)

	for _, name := range repoNames {
		repo := catalog.Catalog[name]
		labels := repo.Labels
		if viper.GetBool(config.SortRepos) && len(labels) > 0 {
			sort.Strings(labels)
		}

		repos = append(repos, repoWithMetadata{
			name:          name,
			description:   repo.Description,
			project:       repo.Project,
			defaultBranch: repo.DefaultBranch,
			labels:        labels,
			isPublic:      repo.Public,
		})
	}

	return catalogModel{
		ctx:   ctx,
		repos: repos,

		printOutput: viper.GetBool(config.PrintResults),
		waitOnExit:  viper.GetBool(config.WaitOnExit),
	}
}

func (m catalogModel) Init() tea.Cmd {
	return nil
}

func (m catalogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m catalogModel) buildContent() string {
	styles := newCatalogStyles(m.width)
	var b strings.Builder

	for i, repo := range m.repos {
		// Repository name
		b.WriteString(styles.repoName.Render(strings.TrimPrefix(repo.name, repo.project+"/")))
		b.WriteString("\n")

		// Description
		if repo.description != "" {
			b.WriteString(styles.description.Render(repo.description))
			b.WriteString("\n")
		}

		// Labels
		if len(repo.labels) > 0 {
			m.buildLabels(&b, repo.labels, styles)
			b.WriteString("\n")
		}

		// Metadata section
		metadata := strings.Builder{}

		// Project
		metadata.WriteString(styles.metaLabel.Render("Project: "))
		metadata.WriteString(styles.metaValue.Render(repo.project))
		metadata.WriteString("  ")

		// Default branch``
		metadata.WriteString(styles.metaLabel.Render("Default Branch: "))
		metadata.WriteString(styles.metaValue.Render(repo.defaultBranch))
		metadata.WriteString("  ")

		// Visibility
		metadata.WriteString(styles.metaLabel.Render("Visibility: "))
		if repo.isPublic {
			metadata.WriteString(styles.publicRepo.Render("public"))
		} else {
			metadata.WriteString(styles.privateRepo.Render("private"))
		}

		b.WriteString(styles.wrap().Render(metadata.String()))
		b.WriteString("\n")

		// Add separator between repos (except for the last one)
		if i < len(m.repos)-1 {
			b.WriteString(styles.separator.Render(separatorLine))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// buildLabels appends the labels line to the provided string builder
func (m catalogModel) buildLabels(b *strings.Builder, labels []string, styles catalogStyles) {
	labelStrs := make([]string, 0, len(labels))
	for _, label := range labels {
		labelStrs = append(labelStrs, styles.label.Render(strings.TrimSpace(label)))
	}

	b.WriteString(styles.wrap().Render("  ( " + strings.Join(labelStrs, ", ") + " )"))
}

func (m catalogModel) View() string {
	if !m.ready {
		return "Loading catalog..."
	}

	styles := newCatalogStyles(m.width)
	var b strings.Builder

	// Title with repository count
	title := fmt.Sprintf("Repository Catalog (%d repositories)", len(m.repos))
	b.WriteString(styles.title.Render(title))
	b.WriteString("\n\n")

	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	b.WriteString(styles.help.Render(footerDone))
	b.WriteString(styles.help.Render(footerVim))

	return b.String()
}

// printFullOutput prints the complete catalog output to stdout, reusing the buildContent logic
func (m catalogModel) printFullOutput(cmd *cobra.Command) {
	styles := newCatalogStyles(m.width)
	out := cmd.OutOrStdout()

	// Title
	title := fmt.Sprintf("Repository Catalog (%d repositories)", len(m.repos))
	fmt.Fprintln(out, styles.title.Render(title))
	fmt.Fprintln(out)

	// Print content (reuses buildContent)
	fmt.Fprint(out, m.buildContent())
}
