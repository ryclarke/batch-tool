package output

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// List of flag names which should be included in the command display for context.
var includeFlags = []string{"script", "branch"}

// BubbleteaHandler is an OutputHandler that uses Bubbletea to provide a modern, interactive interface.
// It displays repository progress with styled output, real-time updates, and a cleaner visual presentation.
func BubbleteaHandler(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error) {
	// Exit early if no repositories are provided
	if len(repos) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), noReposText)
		return
	}

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

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error running bubbletea UI: %v\nUsing default output handler...\n", err)
		// Fallback to native output handling
		NativeHandler(cmd, repos, output, errs)
		return
	}

	// If the user requested to persist output, print it to the terminal
	if m, ok := finalModel.(model); ok && m.printOutput {
		printFullOutput(cmd, m)
	}
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
	printOutput bool
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
	styles      outputStyles
}

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

	// Add flags which add crucial context to the command
	printedFlags := make([]string, 0)
	for _, flagName := range includeFlags {
		if flag := cmd.Flags().Lookup(flagName); flag != nil && flag.Changed {
			printedFlags = append(printedFlags, fmt.Sprintf("%s: `%v`", flagName, flag.Value))
		}
	}

	if len(printedFlags) > 0 {
		cmdParts = append(cmdParts, "("+strings.Join(printedFlags, " ")+")")
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
	m.styles = newOutputStyles(msg.Width)

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

	case "p":
		// Only allow persist output with 'p' after all processing is complete
		if m.allDone {
			m.printOutput = true
			m.quitting = true
			return m, tea.Quit
		}
		fallthrough

	case "esc", "q":
		// Only allow quit with 'esc' or 'q' after all processing is complete
		if m.allDone {
			m.quitting = true
			return m, tea.Quit
		}
		fallthrough

	default:
		// Use shared viewport navigation handler
		if handleKeyPress(&m.viewport, msg.String()) {
			return m, nil
		}

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

// buildContent generates the scrollable content for the viewport
func (m model) buildContent() string {
	return m.buildStyledContent()
}

// buildStyledContent generates styled content for all repositories.
// This function is used by both the viewport (buildContent) and terminal output (printFullOutput).
func (m model) buildStyledContent() string {
	var content strings.Builder

	for i, repo := range m.repos {
		// Add repository section
		content.WriteString(m.formatRepoSection(repo))

		// Add separator between repos (except for the last one)
		if i < len(m.repos)-1 {
			content.WriteString(m.styles.separator.Render(separatorLine))
			content.WriteString("\n")
		}
	}

	return content.String()
}

// printFullOutput prints the complete output to the terminal without viewport wrapping.
// This allows the full output to be persisted after the TUI exits.
func printFullOutput(cmd *cobra.Command, m model) {
	out := cmd.OutOrStdout()
	err := cmd.ErrOrStderr()

	// Print command header
	fmt.Fprintln(err, m.styles.progress.Render(m.command))

	// Print output summary
	progressText := fmt.Sprintf(summaryText, len(m.repos), m.calculateElapsed())
	fmt.Fprintln(err, m.styles.progress.Render(progressText))
	fmt.Fprintln(err)

	// Print all repository outputs using shared formatting logic
	content := m.buildStyledContent()
	fmt.Fprint(out, content)
	fmt.Fprintln(out)
}

// formatRepoSection formats a complete repository section including header, output, and errors.
func (m model) formatRepoSection(repo repoStatus) string {
	var section strings.Builder

	// Repository header
	section.WriteString(m.formatRepoHeader(repo))
	section.WriteString("\n")

	// Show all output lines
	for _, line := range repo.output {
		section.WriteString(m.styles.output.Render(line))
		section.WriteString("\n")
	}

	// Show errors
	for _, errMsg := range repo.errors {
		errorLine := fmt.Sprintf("  ERROR: %s", errMsg.Error())
		section.WriteString(m.styles.outputErr.Render(errorLine))
		section.WriteString("\n")
	}

	return section.String()
}

// formatRepoHeader returns a styled repository header based on its status
func (m model) formatRepoHeader(repo repoStatus) string {
	if repo.completed {
		if repo.failed {
			return m.styles.repoError.Render(fmt.Sprintf(repoErrorFormat, repo.name))
		}

		return m.styles.repoSuccess.Render(fmt.Sprintf(repoSuccessFormat, repo.name))
	}

	if !repo.active {
		// Waiting for concurrency slot to start
		return m.styles.repoWaiting.Render(fmt.Sprintf(repoWaitingFormat, repo.name))
	}

	// Active and running
	return m.styles.repoActive.Render(fmt.Sprintf(repoActiveFormat, repo.name))
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
	b.WriteString(m.styles.progress.Render(m.command))
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

	progressText := fmt.Sprintf(progressText,
		completed, len(m.repos), elapsed)

	b.WriteString(m.styles.progress.Render(progressText))
	b.WriteString("\n")

	// Progress bar
	progressBarWidth := 50
	if m.width > 0 && m.width < 60 {
		progressBarWidth = m.width - 10
	}

	errorCount := m.countErrors()
	progressBar := renderProgressBar(m.styles, completed, errorCount, len(m.repos), progressBarWidth)
	b.WriteString(progressBar)
	b.WriteString(" ")

	percentage := 0
	if len(m.repos) > 0 {
		percentage = (completed * 100) / len(m.repos)
	}

	b.WriteString(m.styles.progress.Render(fmt.Sprintf("%d%%", percentage)))
	b.WriteString("\n")

	return b.String()
}

// renderFooter generates the footer with help text
func (m model) renderFooter() string {
	b := strings.Builder{}
	b.WriteString("\n")

	if m.allDone {
		b.WriteString(m.styles.status.Render(footerDone))
	} else {
		b.WriteString(m.styles.status.Render(footerText))
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

// countErrors returns the number of repositories that completed with errors
func (m model) countErrors() int {
	count := 0
	for _, repo := range m.repos {
		if repo.completed && repo.failed {
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
