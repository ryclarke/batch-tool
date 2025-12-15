package output

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// BubbleteaHandler is an OutputHandler that uses Bubbletea to provide a modern, interactive interface.
// It displays repository progress with styled output, real-time updates, and a cleaner visual presentation.
func BubbleteaHandler(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error) {
	// Create a cancellable context so Ctrl+C can properly cancel subprocesses
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Update the command's context
	cmd.SetContext(ctx)

	p := tea.NewProgram(
		initialModel(cmd, repos, output, errs, cancel),
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error running bubbletea UI: %v\nUsing default output handler...\n", err)
		// Fallback to native output handler on error
		NativeHandler(cmd, repos, output, errs)
	}
}

// color constants
const (
	colorWhite  = "#FFFFFF"
	colorBlue   = "#0000FF"
	colorGreen  = "#04B575"
	colorRed    = "#FF0000"
	colorPurple = "#7D56F4"
	colorCyan   = "#00D4FF"
	colorGray2  = "#222222"
	colorGray4  = "#444444"
	colorGray6  = "#666666"
)

// Styles for the bubbletea UI
var (
	repoActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue)).Bold(true)
	repoWaitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGray6)).Bold(true)
	repoSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true)
	repoErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).Bold(true)

	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGray4))
	outputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite)).MarginLeft(2)
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPurple)).Italic(true)
	progressStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan))

	progressBarCompleteStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorGreen)).
					Background(lipgloss.Color(colorGreen))

	progressBarIncompleteStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorGray4)).
					Background(lipgloss.Color(colorGray2))
)

// repoStatus represents the state of a repository's processing
type repoStatus struct {
	name       string
	output     []string
	errors     []error
	completed  bool
	failed     bool
	active     bool
	outputDone bool
	errorsDone bool
}

// model represents the state of the bubbletea application
type model struct {
	command     string
	repos       []repoStatus
	outputChans []<-chan string
	errChans    []<-chan error
	cancelFunc  context.CancelFunc
	startTime   time.Time
	endTime     time.Time
	quitting    bool
	allDone     bool
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
}

type repoOutputMsg struct {
	index int
	msg   string
}

type repoErrorMsg struct {
	index int
	err   error
}

type repoCompletedMsg struct {
	index int
}

type tickMsg time.Time

func initialModel(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error, cancel context.CancelFunc) model {
	repoStatuses := make([]repoStatus, len(repos))
	for i, repo := range repos {
		repoStatuses[i] = repoStatus{
			name: repo,
		}
	}

	return model{
		command:     buildCommandString(cmd),
		repos:       repoStatuses,
		outputChans: output,
		errChans:    errs,
		cancelFunc:  cancel,
		startTime:   time.Now(),
	}
}

// buildCommandString constructs a display string for the executing command
func buildCommandString(cmd *cobra.Command) string {
	cmdParts := []string{"Executing", cmd.CommandPath()}

	// Add positional arguments
	args := cmd.Flags().Args()
	if len(args) > 0 {
		cmdParts = append(cmdParts, args...)
	}

	// Only add the --script|-c flag if it exists and was set (relevant for `exec` command)
	if scriptFlag := cmd.Flags().Lookup("script"); scriptFlag != nil && scriptFlag.Changed {
		cmdParts = append(cmdParts, fmt.Sprintf("(script: `%s`)", scriptFlag.Value.String()))
	}

	return strings.Join(cmdParts, " ")
}

func (m model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.repos)*2+1)

	// Start listening to all output and error channels
	for i := range m.repos {
		cmds = append(cmds, waitForOutput(i, m.outputChans[i]))
		cmds = append(cmds, waitForError(i, m.errChans[i]))
	}

	// Add ticker for smooth UI updates
	cmds = append(cmds, tickCmd())

	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForOutput(index int, ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return repoCompletedMsg{index: index}
		}

		return repoOutputMsg{index: index, msg: msg}
	}
}

func waitForError(index int, ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-ch
		if !ok {
			return repoCompletedMsg{index: index}
		}

		return repoErrorMsg{index: index, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		// Let viewport handle mouse events for scrolling
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case repoOutputMsg:
		return m.handleRepoOutput(msg)

	case repoErrorMsg:
		return m.handleRepoError(msg)

	case repoCompletedMsg:
		return m.handleRepoCompleted(msg)

	case tickMsg:
		// Continue ticking for smooth UI updates (only when not done)
		if !m.allDone {
			return m, tickCmd()
		}
	}

	return m, nil
}

// handleWindowSize processes window resize events
func (m model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	if !m.ready {
		// Initialize viewport with the terminal size
		headerHeight := 4
		footerHeight := 5
		m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
		m.viewport.YPosition = headerHeight
		m.ready = true
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 9 // Adjust for header and footer
	}

	// Update viewport content with current state
	m.viewport.SetContent(m.buildContent())
	return m, nil
}

// handleKeyPress processes keyboard input
func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c":
		// Cancel context to propagate signal to subprocesses
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		m.quitting = true
		return m, tea.Quit
	case "q":
		// Only allow quit with 'q' after all processing is complete
		if m.allDone {
			m.quitting = true
			return m, tea.Quit
		}
		fallthrough
	default:
		// Let viewport handle all other keys for scrolling
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

// handleRepoOutput processes output messages from repositories
func (m model) handleRepoOutput(msg repoOutputMsg) (tea.Model, tea.Cmd) {
	if msg.index < len(m.repos) {
		// First output signals that the subprocess has started
		if !m.repos[msg.index].active {
			m.repos[msg.index].active = true
			if msg.msg == "" {
				// Skip an initial empty line
				return m, waitForOutput(msg.index, m.outputChans[msg.index])
			}
		}

		m.repos[msg.index].output = append(m.repos[msg.index].output, msg.msg)
	}

	m.viewport.SetContent(m.buildContent())
	return m, waitForOutput(msg.index, m.outputChans[msg.index])
}

// handleRepoError processes error messages from repositories
func (m model) handleRepoError(msg repoErrorMsg) (tea.Model, tea.Cmd) {
	if msg.index < len(m.repos) {
		m.repos[msg.index].errors = append(m.repos[msg.index].errors, msg.err)
	}

	m.viewport.SetContent(m.buildContent())
	return m, waitForError(msg.index, m.errChans[msg.index])
}

// handleRepoCompleted processes completion messages from repositories
func (m model) handleRepoCompleted(msg repoCompletedMsg) (tea.Model, tea.Cmd) {
	if msg.index >= len(m.repos) {
		return m, nil
	}

	// Track which channel closed
	if !m.repos[msg.index].outputDone {
		m.repos[msg.index].outputDone = true
	} else {
		m.repos[msg.index].errorsDone = true
	}

	// Only mark as completed when BOTH channels are closed
	if m.repos[msg.index].outputDone && m.repos[msg.index].errorsDone {
		m.repos[msg.index].completed = true
		m.repos[msg.index].active = false
		// Mark as failed if there were any errors
		if len(m.repos[msg.index].errors) > 0 {
			m.repos[msg.index].failed = true
		}
	}

	// Check if all repositories are done
	if m.allReposCompleted() {
		m.allDone = true
		m.endTime = time.Now()
	}

	m.viewport.SetContent(m.buildContent())
	return m, nil
}

// allReposCompleted checks if all repositories have finished processing
func (m model) allReposCompleted() bool {
	for _, repo := range m.repos {
		if !repo.completed {
			return false
		}
	}

	return true
}

// renderProgressBar creates a visual progress bar
func renderProgressBar(completed, total, width int) string {
	if width < 10 {
		width = 40 // minimum width
	}

	if total == 0 {
		return progressBarIncompleteStyle.Render(strings.Repeat(" ", width))
	}

	percent := float64(completed) / float64(total)
	filledWidth := int(float64(width) * percent)
	emptyWidth := width - filledWidth

	var bar strings.Builder

	if filledWidth > 0 {
		bar.WriteString(progressBarCompleteStyle.Render(strings.Repeat("█", filledWidth)))
	}

	if emptyWidth > 0 {
		bar.WriteString(progressBarIncompleteStyle.Render(strings.Repeat("░", emptyWidth)))
	}

	return bar.String()
}

// buildContent generates the scrollable content for the viewport
func (m model) buildContent() string {
	var content strings.Builder

	for i, repo := range m.repos {
		content.WriteString(m.formatRepoHeader(repo))
		content.WriteString("\n")

		// Show all output
		for _, line := range repo.output {
			content.WriteString(outputStyle.Render(line))
			content.WriteString("\n")
		}

		// Show errors
		for _, errMsg := range repo.errors {
			content.WriteString(outputStyle.Render(fmt.Sprintf("  ERROR: %s", errMsg.Error())))
			content.WriteString("\n")
		}

		// Add separator between repos (except for the last one)
		if i < len(m.repos)-1 {
			content.WriteString(separatorStyle.Render("  ─────────────────────────────────────"))
			content.WriteString("\n")
		}
	}

	return content.String()
}

// formatRepoHeader returns a styled repository header based on its status
func (m model) formatRepoHeader(repo repoStatus) string {
	if repo.completed {
		if repo.failed {
			return repoErrorStyle.Render(fmt.Sprintf("✗ %s", repo.name))
		}

		return repoSuccessStyle.Render(fmt.Sprintf("✓ %s", repo.name))
	}

	if !repo.active {
		// Waiting for concurrency slot to start
		return repoWaitingStyle.Render(fmt.Sprintf("⏸ %s", repo.name))
	}

	// Active and running
	return repoActiveStyle.Render(fmt.Sprintf("▶ %s", repo.name))
}

func (m model) View() string {
	if !m.ready {
		return "Initializing...\n"
	}

	if m.quitting && !m.allDone {
		return "Interrupted.\n"
	}

	var b strings.Builder

	// Command being executed
	b.WriteString(progressStyle.Render(m.command))
	b.WriteString("\n\n")

	// Add viewport to output (content is already set in Update)
	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	// Progress summary and bar
	b.WriteString(m.renderProgress())

	// Footer with scroll hints
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderProgress generates the progress text and bar
func (m model) renderProgress() string {
	var b strings.Builder
	completed := m.countCompleted()
	elapsed := m.calculateElapsed()

	progressText := fmt.Sprintf("Progress: %d/%d repositories | Elapsed: %s",
		completed, len(m.repos), elapsed)

	b.WriteString(progressStyle.Render(progressText))
	b.WriteString("\n")

	// Progress bar
	progressBarWidth := 50
	if m.width > 0 && m.width < 60 {
		progressBarWidth = m.width - 10
	}

	progressBar := renderProgressBar(completed, len(m.repos), progressBarWidth)
	b.WriteString(progressBar)
	b.WriteString(" ")

	percentage := 0
	if len(m.repos) > 0 {
		percentage = (completed * 100) / len(m.repos)
	}

	b.WriteString(progressStyle.Render(fmt.Sprintf("%d%%", percentage)))
	b.WriteString("\n")

	return b.String()
}

// renderFooter generates the footer with help text
func (m model) renderFooter() string {
	b := strings.Builder{}
	b.WriteString("\n")

	if m.allDone {
		b.WriteString(statusStyle.Render("✓ All repositories processed! Use ↑/↓ or j/k to scroll, q or Ctrl+C to quit"))
	} else {
		b.WriteString(statusStyle.Render("Use ↑/↓ or j/k to scroll | Ctrl+C to interrupt"))
	}
	b.WriteString("\n")

	return b.String()
}

// countCompleted returns the number of completed repositories
func (m model) countCompleted() int {
	count := 0
	for _, repo := range m.repos {
		if repo.completed {
			count++
		}
	}

	return count
}

// calculateElapsed returns the elapsed time, using endTime if all done
func (m model) calculateElapsed() time.Duration {
	if m.allDone {
		return m.endTime.Sub(m.startTime).Round(time.Second)
	}

	return time.Since(m.startTime).Round(time.Second)
}
