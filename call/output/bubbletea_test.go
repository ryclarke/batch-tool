package output

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// testCancelFunc is a no-op cancel function for tests
func testCancelFunc() { /* no-op */ }

// makeClosedChannels creates closed channels for testing to avoid blocking
func makeClosedChannels(count int) ([]<-chan string, []<-chan error) {
	outputChans := make([]<-chan string, count)
	errChans := make([]<-chan error, count)
	for i := 0; i < count; i++ {
		outCh := make(chan string)
		errCh := make(chan error)
		close(outCh)
		close(errCh)
		outputChans[i] = outCh
		errChans[i] = errCh
	}
	return outputChans, errChans
}

// TestBuildCommandString tests the command string building logic
func TestBuildCommandString(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*cobra.Command)
		expected string
	}{
		{
			name: "basic command with no args",
			setup: func(cmd *cobra.Command) {
				cmd.Use = "test"
			},
			expected: "Executing test",
		},
		{
			name: "command with positional args",
			setup: func(cmd *cobra.Command) {
				cmd.Use = "test"
				cmd.SetArgs([]string{"arg1", "arg2"})
				cmd.ParseFlags([]string{"arg1", "arg2"})
			},
			expected: "Executing test arg1 arg2",
		},
		{
			name: "command with script flag",
			setup: func(cmd *cobra.Command) {
				cmd.Use = "test"
				cmd.Flags().String("script", "", "script path")
				cmd.SetArgs([]string{"--script=test.sh"})
				cmd.ParseFlags([]string{"--script=test.sh"})
			},
			expected: "Executing test (script: `test.sh`)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			tt.setup(cmd)

			result := buildCommandString(cmd)
			if result != tt.expected {
				t.Errorf("buildCommandString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestInitialModel tests the model initialization
func TestInitialModel(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2", "repo3"}
	outputChans, errChans := makeClosedChannels(len(repos))

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	if len(m.repos) != 3 {
		t.Errorf("Expected 3 repos, got %d", len(m.repos))
	}

	for i, repo := range m.repos {
		if repo.name != repos[i] {
			t.Errorf("Expected repo name %s, got %s", repos[i], repo.name)
		}
		if repo.active {
			t.Errorf("Expected repo %s to be inactive (waiting) initially", repo.name)
		}
		if repo.completed {
			t.Errorf("Expected repo %s to not be completed", repo.name)
		}
	}

	if m.startTime.IsZero() {
		t.Error("Expected startTime to be set")
	}

	if !strings.Contains(m.command, "Executing") {
		t.Errorf("Expected command to contain 'Executing', got %s", m.command)
	}
}

// TestHandleRepoOutput tests output message handling
func TestHandleRepoOutput(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}
	outputChans, errChans := makeClosedChannels(len(repos))

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	msg := repoOutputMsg{index: 0, msg: "test output"}
	newModel, _ := m.handleRepoOutput(msg)
	m = newModel.(model)

	if len(m.repos[0].output) != 1 {
		t.Errorf("Expected 1 output line, got %d", len(m.repos[0].output))
	}

	if m.repos[0].output[0] != "test output" {
		t.Errorf("Expected 'test output', got %s", m.repos[0].output[0])
	}

	// After first output, repo should transition from inactive to active
	if !m.repos[0].active {
		t.Error("Expected repo to be active after first output")
	}
}

// TestHandleRepoError tests error message handling
func TestHandleRepoError(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	msg := repoErrorMsg{index: 0, err: errors.New("test error")}
	newModel, _ := m.handleRepoError(msg)
	m = newModel.(model)

	if len(m.repos[0].errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(m.repos[0].errors))
	}

	if m.repos[0].errors[0].Error() != "test error" {
		t.Errorf("Expected 'test error', got %s", m.repos[0].errors[0].Error())
	}

	close(out)
	close(err)
}

// TestHandleRepoCompleted tests completion message handling
func TestHandleRepoCompleted(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	// First completion (output channel closed)
	msg := repoCompletedMsg{index: 0}
	newModel, _ := m.handleRepoCompleted(msg)
	m = newModel.(model)

	if !m.repos[0].outputDone {
		t.Error("Expected outputDone to be true")
	}
	if m.repos[0].completed {
		t.Error("Expected repo to not be completed yet (waiting for error channel)")
	}

	// Second completion (error channel closed)
	newModel, _ = m.handleRepoCompleted(msg)
	m = newModel.(model)

	if !m.repos[0].errorsDone {
		t.Error("Expected errorsDone to be true")
	}
	if !m.repos[0].completed {
		t.Error("Expected repo to be completed")
	}
	if m.repos[0].active {
		t.Error("Expected repo to not be active")
	}

	close(out)
	close(err)
}

// TestAllReposCompleted tests the completion check logic
func TestAllReposCompleted(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2"}

	out1, out2 := make(chan string), make(chan string)
	err1, err2 := make(chan error), make(chan error)
	outputChans := []<-chan string{out1, out2}
	errChans := []<-chan error{err1, err2}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	if m.allReposCompleted() {
		t.Error("Expected allReposCompleted to be false initially")
	}

	m.repos[0].completed = true
	if m.allReposCompleted() {
		t.Error("Expected allReposCompleted to be false with only one repo completed")
	}

	m.repos[1].completed = true
	if !m.allReposCompleted() {
		t.Error("Expected allReposCompleted to be true with all repos completed")
	}

	close(out1)
	close(out2)
	close(err1)
	close(err2)
}

// TestCountCompleted tests the count completed function
func TestCountCompleted(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2", "repo3"}

	out := make([]<-chan string, 3)
	err := make([]<-chan error, 3)
	for i := range out {
		outChan := make(chan string)
		errChan := make(chan error)
		out[i] = outChan
		err[i] = errChan
		defer close(outChan)
		defer close(errChan)
	}

	m := initialModel(cmd, repos, out, err, testCancelFunc)

	if count := m.countCompleted(); count != 0 {
		t.Errorf("Expected 0 completed repos, got %d", count)
	}

	m.repos[0].completed = true
	if count := m.countCompleted(); count != 1 {
		t.Errorf("Expected 1 completed repo, got %d", count)
	}

	m.repos[1].completed = true
	m.repos[2].completed = true
	if count := m.countCompleted(); count != 3 {
		t.Errorf("Expected 3 completed repos, got %d", count)
	}
}

// TestCountErrors tests the count errors function
func TestCountErrors(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2", "repo3"}

	out := make([]<-chan string, 3)
	err := make([]<-chan error, 3)
	for i := range out {
		outChan := make(chan string)
		errChan := make(chan error)
		out[i] = outChan
		err[i] = errChan
		defer close(outChan)
		defer close(errChan)
	}

	m := initialModel(cmd, repos, out, err, testCancelFunc)

	if count := m.countErrors(); count != 0 {
		t.Errorf("Expected 0 errors, got %d", count)
	}

	// Completed but no errors
	m.repos[0].completed = true
	m.repos[0].failed = false
	if count := m.countErrors(); count != 0 {
		t.Errorf("Expected 0 errors for completed successful repo, got %d", count)
	}

	// Completed with errors
	m.repos[1].completed = true
	m.repos[1].failed = true
	if count := m.countErrors(); count != 1 {
		t.Errorf("Expected 1 error, got %d", count)
	}

	// Not completed with failed flag (shouldn't count)
	m.repos[2].completed = false
	m.repos[2].failed = true
	if count := m.countErrors(); count != 1 {
		t.Errorf("Expected 1 error (incomplete repos shouldn't count), got %d", count)
	}

	// Complete the third repo
	m.repos[2].completed = true
	if count := m.countErrors(); count != 2 {
		t.Errorf("Expected 2 errors, got %d", count)
	}
}

// TestCalculateElapsed tests the elapsed time calculation
func TestCalculateElapsed(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}
	outputChans, errChans := makeClosedChannels(len(repos))

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.startTime = time.Now().Add(-5 * time.Second)

	// Test ongoing calculation
	elapsed := m.calculateElapsed()
	if elapsed < 4*time.Second || elapsed > 6*time.Second {
		t.Errorf("Expected elapsed time around 5s, got %v", elapsed)
	}

	// Test completed calculation
	m.allDone = true
	m.endTime = m.startTime.Add(3 * time.Second)
	elapsed = m.calculateElapsed()
	if elapsed != 3*time.Second {
		t.Errorf("Expected elapsed time of 3s, got %v", elapsed)
	}
}

// TestFormatRepoHeader tests repository header formatting
func TestFormatRepoHeader(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}
	outputChans, errChans := makeClosedChannels(len(repos))

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	tests := []struct {
		name     string
		setup    func(*repoStatus)
		contains string
	}{
		{
			name: "waiting repo",
			setup: func(r *repoStatus) {
				r.active = false
				r.completed = false
			},
			contains: "⏸",
		},
		{
			name: "active repo",
			setup: func(r *repoStatus) {
				r.active = true
				r.completed = false
			},
			contains: "▶",
		},
		{
			name: "completed successful repo",
			setup: func(r *repoStatus) {
				r.active = false
				r.completed = true
				r.failed = false
			},
			contains: "✓",
		},
		{
			name: "completed failed repo",
			setup: func(r *repoStatus) {
				r.active = false
				r.completed = true
				r.failed = true
			},
			contains: "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repoStatus{name: "test-repo"}
			tt.setup(&repo)

			result := m.formatRepoHeader(repo)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected header to contain %s, got %s", tt.contains, result)
			}
			if !strings.Contains(result, "test-repo") {
				t.Errorf("Expected header to contain repo name, got %s", result)
			}
		})
	}
}

// TestRenderProgressBar tests the progress bar rendering
func TestRenderProgressBar(t *testing.T) {
	styles := newOutputStyles(80) // Create styles for testing
	tests := []struct {
		name      string
		completed int
		errors    int
		total     int
		width     int
		check     func(string) bool
	}{
		{
			name:      "zero progress",
			completed: 0,
			errors:    0,
			total:     10,
			width:     20,
			check: func(s string) bool {
				return strings.Contains(s, "░")
			},
		},
		{
			name:      "full progress no errors",
			completed: 10,
			errors:    0,
			total:     10,
			width:     20,
			check: func(s string) bool {
				return strings.Contains(s, "█")
			},
		},
		{
			name:      "partial progress",
			completed: 5,
			errors:    0,
			total:     10,
			width:     20,
			check: func(s string) bool {
				return strings.Contains(s, "█") && strings.Contains(s, "░")
			},
		},
		{
			name:      "progress with errors",
			completed: 10,
			errors:    3,
			total:     10,
			width:     20,
			check: func(s string) bool {
				// Should contain both success and error indicators
				return strings.Contains(s, "█")
			},
		},
		{
			name:      "zero total",
			completed: 0,
			errors:    0,
			total:     0,
			width:     20,
			check: func(s string) bool {
				return len(s) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderProgressBar(styles, tt.completed, tt.errors, tt.total, tt.width)
			if !tt.check(result) {
				t.Errorf("Progress bar check failed for %s: %s", tt.name, result)
			}
		})
	}
}

// TestBuildContent tests content building
func TestBuildContent(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2"}

	out1, out2 := make(chan string), make(chan string)
	err1, err2 := make(chan error), make(chan error)
	outputChans := []<-chan string{out1, out2}
	errChans := []<-chan error{err1, err2}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	// Add some output and errors
	m.repos[0].output = []string{"line 1", "line 2"}
	m.repos[0].errors = []error{errors.New("error 1")}
	m.repos[1].output = []string{"line 3"}

	content := m.buildContent()

	// Check that content contains repo names
	if !strings.Contains(content, "repo1") {
		t.Error("Expected content to contain repo1")
	}
	if !strings.Contains(content, "repo2") {
		t.Error("Expected content to contain repo2")
	}

	// Check that content contains output
	if !strings.Contains(content, "line 1") {
		t.Error("Expected content to contain 'line 1'")
	}

	// Check that content contains errors
	if !strings.Contains(content, "error 1") {
		t.Error("Expected content to contain 'error 1'")
	}

	// Check separator exists between repos
	if !strings.Contains(content, "─") {
		t.Error("Expected content to contain separator")
	}

	close(out1)
	close(out2)
	close(err1)
	close(err2)
}

// TestWaitForOutput tests the output channel waiting function
func TestWaitForOutput(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "test message"

	cmd := waitForOutput(0, ch)
	msg := cmd()

	if outputMsg, ok := msg.(repoOutputMsg); ok {
		if outputMsg.index != 0 {
			t.Errorf("Expected index 0, got %d", outputMsg.index)
		}
		if outputMsg.msg != "test message" {
			t.Errorf("Expected 'test message', got %s", outputMsg.msg)
		}
	} else {
		t.Errorf("Expected repoOutputMsg, got %T", msg)
	}

	// Test channel close
	close(ch)
	cmd = waitForOutput(0, ch)
	msg = cmd()

	if completedMsg, ok := msg.(repoCompletedMsg); ok {
		if completedMsg.index != 0 {
			t.Errorf("Expected index 0, got %d", completedMsg.index)
		}
	} else {
		t.Errorf("Expected repoCompletedMsg, got %T", msg)
	}
}

// TestWaitForError tests the error channel waiting function
func TestWaitForError(t *testing.T) {
	ch := make(chan error, 1)
	testErr := errors.New("test error")
	ch <- testErr

	cmd := waitForError(0, ch)
	msg := cmd()

	if errMsg, ok := msg.(repoErrorMsg); ok {
		if errMsg.index != 0 {
			t.Errorf("Expected index 0, got %d", errMsg.index)
		}
		if errMsg.err.Error() != "test error" {
			t.Errorf("Expected 'test error', got %s", errMsg.err.Error())
		}
	} else {
		t.Errorf("Expected repoErrorMsg, got %T", msg)
	}

	// Test channel close
	close(ch)
	cmd = waitForError(0, ch)
	msg = cmd()

	if completedMsg, ok := msg.(repoCompletedMsg); ok {
		if completedMsg.index != 0 {
			t.Errorf("Expected index 0, got %d", completedMsg.index)
		}
	} else {
		t.Errorf("Expected repoCompletedMsg, got %T", msg)
	}
}

// TestHandleRepoCompletedSetsAllDone tests that all done is set when all repos complete
func TestHandleRepoCompletedSetsAllDone(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	// Complete both channels
	msg := repoCompletedMsg{index: 0}
	newModel, _ := m.handleRepoCompleted(msg)
	m = newModel.(model)
	newModel, _ = m.handleRepoCompleted(msg)
	m = newModel.(model)

	if !m.allDone {
		t.Error("Expected allDone to be true after all repos complete")
	}

	if m.endTime.IsZero() {
		t.Error("Expected endTime to be set when all done")
	}

	close(out)
	close(err)
}

// TestHandleRepoCompletedWithError tests that failed repos are marked
func TestHandleRepoCompletedWithError(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	// Add an error
	m.repos[0].errors = []error{errors.New("test error")}

	// Complete both channels
	msg := repoCompletedMsg{index: 0}
	newModel, _ := m.handleRepoCompleted(msg)
	m = newModel.(model)
	newModel, _ = m.handleRepoCompleted(msg)
	m = newModel.(model)

	if !m.repos[0].failed {
		t.Error("Expected repo with errors to be marked as failed")
	}

	close(out)
	close(err)
}

// TestHandleRepoCompletedOutOfBounds tests handling completion for invalid index
func TestHandleRepoCompletedOutOfBounds(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.viewport = viewport.New(80, 24)

	// Try to complete with invalid index
	msg := repoCompletedMsg{index: 999}
	newModel, _ := m.handleRepoCompleted(msg)
	m = newModel.(model)

	// Should not crash
	if m.repos[0].completed {
		t.Error("Expected repo to not be marked completed for out of bounds index")
	}

	close(out)
	close(err)
}

// TestFormatRepoHeaderInactive tests inactive repo formatting
func TestFormatRepoHeaderInactive(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	repo := repoStatus{
		name:      "test-repo",
		active:    false,
		completed: false,
	}

	result := m.formatRepoHeader(repo)

	// Should just be the repo name
	if !strings.Contains(result, "test-repo") {
		t.Errorf("Expected header to contain repo name, got %s", result)
	}

	close(out)
	close(err)
}

// TestRenderProgressBarSmallWidth tests minimum width handling
func TestRenderProgressBarSmallWidth(t *testing.T) {
	styles := newOutputStyles(80)
	result := renderProgressBar(styles, 5, 0, 10, 5)
	// Should use minimum width of 40
	if len(result) < 10 {
		t.Errorf("Expected progress bar to use minimum width, got length %d", len(result))
	}
}

// TestModelView tests the View rendering in different states
func TestModelView(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	// Test not ready state
	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Error("Expected view to show 'Initializing' when not ready")
	}

	// Setup ready state
	m.ready = true
	m.viewport = viewport.New(80, 24)
	m.width = 80
	m.height = 24

	// Test normal view
	view = m.View()
	if !strings.Contains(view, "Executing") {
		t.Error("Expected view to contain 'Executing'")
	}

	// Test interrupted state
	m.quitting = true
	m.allDone = false
	view = m.View()
	if !strings.Contains(view, "Interrupted") {
		t.Error("Expected view to show 'Interrupted'")
	}

	close(out)
	close(err)
}

// TestRenderProgress tests progress rendering
func TestRenderProgress(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2"}

	out1, out2 := make(chan string), make(chan string)
	err1, err2 := make(chan error), make(chan error)
	outputChans := []<-chan string{out1, out2}
	errChans := []<-chan error{err1, err2}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.width = 80
	m.repos[0].completed = true

	progress := m.renderProgress()

	if !strings.Contains(progress, "Progress:") {
		t.Error("Expected progress to contain 'Progress:'")
	}
	if !strings.Contains(progress, "1/2") {
		t.Error("Expected progress to show '1/2 repositories'")
	}
	if !strings.Contains(progress, "50%") {
		t.Error("Expected progress to show '50%'")
	}

	close(out1)
	close(out2)
	close(err1)
	close(err2)
}

// TestRenderFooter tests footer rendering
func TestRenderFooter(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	// Test footer while processing
	footer := m.renderFooter()
	if !strings.Contains(footer, "↑/↓: scroll") {
		t.Error("Expected footer to show scroll instructions while processing")
	}
	if !strings.Contains(footer, "supports Vim keybinds") {
		t.Error("Expected footer to mention vim key support")
	}

	// Test footer when done
	m.allDone = true
	footer = m.renderFooter()
	if !strings.Contains(footer, "All done") {
		t.Error("Expected footer to show completion message when done")
	}
	if !strings.Contains(footer, "q: quit") {
		t.Error("Expected footer to show quit instructions when done")
	}

	close(out)
	close(err)
}

// TestModelInit tests the Init function
func TestModelInit(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1", "repo2"}

	out1, out2 := make(chan string), make(chan string)
	err1, err2 := make(chan error), make(chan error)
	outputChans := []<-chan string{out1, out2}
	errChans := []<-chan error{err1, err2}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Close channels to prevent goroutine leaks
	close(out1)
	close(out2)
	close(err1)
	close(err2)
}

// TestHandleWindowSize tests window size handling
func TestHandleWindowSize(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)

	// Test initial window size (viewport not ready)
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := m.handleWindowSize(msg)
	m = newModel.(model)

	if !m.ready {
		t.Error("Expected model to be ready after window size message")
	}
	if m.width != 100 {
		t.Errorf("Expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height 50, got %d", m.height)
	}

	// Test subsequent window size (resize)
	msg = tea.WindowSizeMsg{Width: 120, Height: 60}
	newModel, _ = m.handleWindowSize(msg)
	m = newModel.(model)

	if m.width != 120 {
		t.Errorf("Expected width 120, got %d", m.width)
	}
	if m.height != 60 {
		t.Errorf("Expected height 60, got %d", m.height)
	}

	close(out)
	close(err)
}

// TestHandleKeyPressCtrlC tests Ctrl+C handling
func TestHandleKeyPressCtrlC(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	// Track if cancel was called
	cancelCalled := false
	cancelFunc := func() { cancelCalled = true }

	m := initialModel(cmd, repos, outputChans, errChans, cancelFunc)
	m.ready = true
	m.viewport = viewport.New(80, 24)

	// Create a key message for Ctrl+C
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(model)

	if !m.quitting {
		t.Error("Expected Ctrl+C to set quitting to true")
	}

	if !cancelCalled {
		t.Error("Expected Ctrl+C to call cancel function")
	}

	close(out)
	close(err)
}

// TestHandleKeyPressQ tests 'q' key handling
func TestHandleKeyPressQ(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.ready = true
	m.viewport = viewport.New(80, 24)

	// Test 'q' when not done - should not quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(model)

	if m.quitting {
		t.Error("Expected 'q' to not quit when processing is not done")
	}

	// Test 'q' when done - should quit
	m.allDone = true
	newModel, _ = m.handleKeyPress(msg)
	m = newModel.(model)

	if !m.quitting {
		t.Error("Expected 'q' to quit when all processing is done")
	}

	close(out)
	close(err)
}

// TestHandleKeyPressP tests 'p' key handling for persist output
func TestHandleKeyPressP(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	repos := []string{"repo1"}

	out := make(chan string)
	err := make(chan error)
	outputChans := []<-chan string{out}
	errChans := []<-chan error{err}

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.ready = true
	m.viewport = viewport.New(80, 24)

	// Test 'p' when not done - should not quit or persist
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newModel, _ := m.handleKeyPress(msg)
	m = newModel.(model)

	if m.quitting {
		t.Error("Expected 'p' to not quit when processing is not done")
	}
	if m.printOutput {
		t.Error("Expected 'p' to not set persistAfter when processing is not done")
	}

	// Test 'p' when done - should quit and persist
	m.allDone = true
	newModel, _ = m.handleKeyPress(msg)
	m = newModel.(model)

	if !m.quitting {
		t.Error("Expected 'p' to quit when all processing is done")
	}
	if !m.printOutput {
		t.Error("Expected 'p' to set persistAfter when all processing is done")
	}

	close(out)
	close(err)
}

// TestPrintFullOutput tests the printFullOutput function
func TestPrintFullOutput(t *testing.T) {
	var output strings.Builder
	var errOut strings.Builder
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&output)
	cmd.SetErr(&errOut)

	repos := []string{"repo1", "repo2"}
	outputChans, errChans := makeClosedChannels(2)

	m := initialModel(cmd, repos, outputChans, errChans, testCancelFunc)
	m.allDone = true
	m.endTime = m.startTime.Add(2 * time.Second)

	// Set up some test output
	m.repos[0].completed = true
	m.repos[0].failed = false
	m.repos[0].output = []string{"line 1", "line 2"}

	m.repos[1].completed = true
	m.repos[1].failed = true
	m.repos[1].output = []string{"error line"}
	m.repos[1].errors = []error{errors.New("test error")}

	printFullOutput(cmd, m)

	result := output.String()
	errs := errOut.String()

	// Verify the output contains expected elements
	if !strings.Contains(errs, "Executing test") {
		t.Error("Expected output to contain command string (in stderr)")
	}
	if !strings.Contains(errs, "2 repositories") {
		t.Error("Expected output to contain summary (in stderr)")
	}
	if !strings.Contains(result, "✓ repo1") {
		t.Error("Expected output to contain successful repo header")
	}
	if !strings.Contains(result, "✗ repo2") {
		t.Error("Expected output to contain failed repo header")
	}
	if !strings.Contains(result, "line 1") {
		t.Error("Expected output to contain repo1 output")
	}
	if !strings.Contains(result, "line 2") {
		t.Error("Expected output to contain repo1 output")
	}
	if !strings.Contains(result, "error line") {
		t.Error("Expected output to contain repo2 output")
	}
	if !strings.Contains(result, "ERROR: test error") {
		t.Error("Expected output to contain error message (from subroutine, as stdout)")
	}
}

// TestTickCmd tests the tick command
func TestTickCmd(t *testing.T) {
	cmd := tickCmd()
	if cmd == nil {
		t.Fatal("Expected tickCmd to return a command")
	}
}
