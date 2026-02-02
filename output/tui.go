package output

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
)

// List of flag names which should be included in the command display for context.
var includeFlags = []string{"script", "file", "arg", "branch", "reviewer"}

// TUIHandler is an OutputHandler that uses a TUI to provide a modern, interactive interface.
// It displays repository progress with styled output, real-time updates, and a cleaner visual presentation.
func TUIHandler(cmd *cobra.Command, channels []Channel) {
	// Exit early if no repositories are provided
	if len(channels) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), noReposText)
		return
	}

	// Create a cancellable context so Ctrl+C can properly cancel subprocesses
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Update the command's context
	cmd.SetContext(ctx)

	p := tea.NewProgram(
		initialModel(cmd, channels, cancel),
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Ensure terminal is restored on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt to restore terminal state
			p.Kill()
			// Print panic info after terminal is restored
			fmt.Fprintf(cmd.ErrOrStderr(), "\nTUI panic recovered: %v\n", r)
		}
	}()

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), tuiFailText, err)
		// Fallback to native output handling
		NativeHandler(cmd, channels)
		return
	}

	// If the user requested to persist output, print it to the terminal
	if m, ok := finalModel.(*model); ok && m.printOutput {
		printFullOutput(cmd, m)
	}
}

// model represents the state of the TUI application
type model struct {
	mu sync.RWMutex

	command    string
	repos      []*repoStatus
	cancelFunc context.CancelFunc
	startTime  time.Time
	endTime    time.Time
	quitting   bool
	allDone    bool
	viewport   viewport.Model
	ready      bool
	width      int
	height     int
	styles     outputStyles

	printOutput bool
	waitOnExit  bool
}

// repoStatus represents the state of a repository's processing
type repoStatus struct {
	Channel

	output     []byte
	errors     []error
	completed  bool
	failed     bool
	active     bool
	outputDone bool
	errorsDone bool
}

type repoOutputMsg struct {
	index int
	data  []byte
}

type repoErrorMsg struct {
	index int
	err   error
}

type repoCompletedMsg struct {
	index int
}

type tickMsg time.Time

func initialModel(cmd *cobra.Command, channels []Channel, cancel context.CancelFunc) *model {
	viper := config.Viper(cmd.Context())

	repoStatuses := make([]*repoStatus, len(channels))
	for i, ch := range channels {
		repoStatuses[i] = &repoStatus{
			Channel: ch,
		}
	}

	output := make([]<-chan []byte, len(channels))
	errs := make([]<-chan error, len(channels))
	for i, ch := range channels {
		output[i] = ch.Out()
		errs[i] = ch.Err()
	}

	return &model{
		command:    buildCommandString(cmd),
		repos:      repoStatuses,
		cancelFunc: cancel,
		startTime:  time.Now(),

		printOutput: viper.GetBool(config.PrintResults),
		waitOnExit:  viper.GetBool(config.WaitOnExit),
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

func (m *model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.repos)*2+1)

	// Start listening to all output and error channels
	for i := range m.repos {
		cmds = append(cmds, waitForOutput(i, m.repos[i]))
		cmds = append(cmds, waitForError(i, m.repos[i]))
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

func waitForOutput(index int, ch Channel) tea.Cmd {
	return func() tea.Msg {
		data, ok := <-ch.Out()
		if !ok {
			return repoCompletedMsg{index: index}
		}

		return repoOutputMsg{index: index, data: data}
	}
}

func waitForError(index int, ch Channel) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-ch.Err()
		if !ok {
			return repoCompletedMsg{index: index}
		}

		return repoErrorMsg{index: index, err: err}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
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
func (m *model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		// Only allow print and quit with 'p' after all processing is complete
		if m.allDone {
			m.printOutput = true
			m.quitting = true
			return m, tea.Quit
		}
		fallthrough

	case "enter", "esc", "q":
		// Only allow quit with 'enter', 'esc' or 'q' after all processing is complete
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
func (m *model) handleRepoOutput(msg repoOutputMsg) (tea.Model, tea.Cmd) {
	ch, skip := m.appendRepoOutput(msg.index, msg.data)
	if ch == nil {
		return m, nil
	}

	if !skip {
		m.viewport.SetContent(m.buildContent())
	}

	return m, waitForOutput(msg.index, ch)
}

// handleRepoError processes error messages from repositories
func (m *model) handleRepoError(msg repoErrorMsg) (tea.Model, tea.Cmd) {
	ch := m.appendRepoError(msg.index, msg.err)
	if ch == nil {
		return m, nil
	}

	m.viewport.SetContent(m.buildContent())
	return m, waitForError(msg.index, ch)
}

// handleRepoCompleted processes completion messages from repositories
func (m *model) handleRepoCompleted(msg repoCompletedMsg) (tea.Model, tea.Cmd) {
	allDone, valid := m.markRepoChannelClosed(msg.index)
	if !valid {
		return m, nil
	}

	if allDone {
		m.allDone = true
		m.endTime = time.Now()

		// Auto-quit if wait flag is false
		if !m.waitOnExit {
			m.quitting = true
			return m, tea.Quit
		}
	}

	m.viewport.SetContent(m.buildContent())
	return m, nil
}

// appendRepoOutput appends data to a repo's output buffer.
// Returns the channel and whether the message should be skipped (initial empty line).
// Returns nil channel if index is out of bounds.
func (m *model) appendRepoOutput(index int, data []byte) (ch Channel, skip bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index >= len(m.repos) {
		return nil, false
	}

	// First output signals that the subprocess has started
	if !m.repos[index].active {
		m.repos[index].active = true
		if len(data) == 0 {
			// Skip an initial empty line
			return m.repos[index].Channel, true
		}
	}

	m.repos[index].output = append(m.repos[index].output, data...)
	return m.repos[index].Channel, false
}

// appendRepoError appends an error to a repo's error list.
// Returns the channel, or nil if index is out of bounds.
func (m *model) appendRepoError(index int, err error) Channel {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index >= len(m.repos) {
		return nil
	}

	m.repos[index].errors = append(m.repos[index].errors, err)
	return m.repos[index].Channel
}

// markRepoChannelClosed marks a channel as closed and checks completion state.
// Returns (allDone, valid) where valid is false if index is out of bounds.
func (m *model) markRepoChannelClosed(index int) (allDone, valid bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index >= len(m.repos) {
		return false, false
	}

	// Track which channel closed
	if !m.repos[index].outputDone {
		m.repos[index].outputDone = true
	} else {
		m.repos[index].errorsDone = true
	}

	// Only mark as completed when BOTH channels are closed
	if m.repos[index].outputDone && m.repos[index].errorsDone {
		m.repos[index].completed = true
		m.repos[index].active = false
		// Mark as failed if there were any errors
		if len(m.repos[index].errors) > 0 {
			m.repos[index].failed = true
		}
	}

	// Check if all repositories are done
	for _, repo := range m.repos {
		if !repo.completed {
			return false, true
		}
	}

	return true, true
}

// allReposCompleted checks if all repositories have finished processing.
func (m *model) allReposCompleted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, repo := range m.repos {
		if !repo.completed {
			return false
		}
	}

	return true
}

// buildContent generates the scrollable content for the viewport
func (m *model) buildContent() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.buildContentUnsafe()
}

func (m *model) buildContentUnsafe() string {
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
func printFullOutput(cmd *cobra.Command, m *model) {
	out := cmd.OutOrStdout()
	err := cmd.ErrOrStderr()

	// Print command header
	fmt.Fprintln(err, m.styles.progress.Render(m.command))

	// Print output summary
	progressSummary := fmt.Sprintf(summaryText, len(m.repos), m.getDuration())
	fmt.Fprintln(err, m.styles.progress.Render(progressSummary))
	fmt.Fprintln(err)

	// Print all repository outputs using shared formatting logic
	content := m.buildContent()
	fmt.Fprint(out, content)
	fmt.Fprintln(out)
}

// formatRepoSection formats a complete repository section including header, output, and errors.
func (m *model) formatRepoSection(repo *repoStatus) string {
	var section strings.Builder

	// Repository header
	section.WriteString(m.formatRepoHeader(repo))
	section.WriteString("\n")

	// Split output stream by newlines
	for line := range bytes.SplitSeq(repo.output, []byte{'\n'}) {
		if len(line) > 0 {
			section.WriteString(m.styles.output.Render(string(line)))
		}
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
func (m *model) formatRepoHeader(repo *repoStatus) string {
	if repo.completed {
		if repo.failed {
			return m.styles.repoError.Render(fmt.Sprintf(repoErrorFormat, repo.Name()))
		}

		return m.styles.repoSuccess.Render(fmt.Sprintf(repoSuccessFormat, repo.Name()))
	}

	if !repo.active {
		// Waiting for concurrency slot to start
		return m.styles.repoWaiting.Render(fmt.Sprintf(repoWaitingFormat, repo.Name()))
	}

	// Active and running
	return m.styles.repoActive.Render(fmt.Sprintf(repoActiveFormat, repo.Name()))
}

func (m *model) View() string {
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
func (m *model) renderProgress() string {
	var b strings.Builder
	completed := m.countCompleted()
	failed := m.countFailed()
	elapsed := m.getDuration()

	var progressSummary string

	if failed > 0 {
		progressSummary = fmt.Sprintf(progressTextFail,
			completed, len(m.repos), failed, elapsed)
	} else {
		progressSummary = fmt.Sprintf(progressText,
			completed, len(m.repos), elapsed)
	}

	b.WriteString(m.styles.progress.Render(progressSummary))
	b.WriteString("\n")

	// Progress bar
	progressBarWidth := 50
	if m.width > 0 && m.width < 60 {
		progressBarWidth = m.width - 10
	}

	progressBar := renderProgressBar(m.styles, completed, failed, len(m.repos), progressBarWidth)
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
func (m *model) renderFooter() string {
	b := strings.Builder{}
	b.WriteString("\n")

	if m.allDone {
		b.WriteString(m.styles.status.Render(footerDone))
		b.WriteString(m.styles.status.Render(footerVim))
	} else {
		b.WriteString(m.styles.status.Render(footerText))
		b.WriteString(m.styles.status.Render(footerVim))
	}
	b.WriteString("\n")

	return b.String()
}

// countCompleted returns the number of completed repositories
func (m *model) countCompleted() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.countCompletedUnsafe()
}

// countCompletedUnsafe returns the count (caller must hold at least RLock).
func (m *model) countCompletedUnsafe() int {
	count := 0
	for _, repo := range m.repos {
		if repo.completed {
			count++
		}
	}

	return count
}

// countFailed returns the number of repositories that completed with errors
func (m *model) countFailed() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.countFailedUnsafe()
}

func (m *model) countFailedUnsafe() int {
	count := 0
	for _, repo := range m.repos {
		if repo.completed && repo.failed {
			count++
		}
	}

	return count
}

// getDuration returns the elapsed time, using endTime if all done
func (m *model) getDuration() time.Duration {
	if m.allDone {
		return m.endTime.Sub(m.startTime).Round(time.Second)
	}

	return time.Since(m.startTime).Round(time.Second)
}
