package call

import (
	"os"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

// TestWrap tests the Wrap function which combines multiple CallFuncs
func TestWrap(t *testing.T) {
	ctx := loadFixture(t)

	testRepo := "test-repo"
	output1 := "output1 from " + testRepo
	output2 := "output2 from " + testRepo

	tests := []struct {
		name       string
		callFuncs  []Func
		repo       string
		wantOutput []string
		wantError  bool
	}{
		{
			name: "basic wrap with two functions",
			callFuncs: []Func{
				fakeCallFunc(t, false, output1),
				fakeCallFunc(t, false, output2),
			},
			repo:       testRepo,
			wantOutput: []string{output1, output2},
		},
		{
			name: "wrap with error in first function",
			callFuncs: []Func{
				fakeCallFunc(t, true, output1),
				fakeCallFunc(t, false, output2),
			},
			repo:       testRepo,
			wantOutput: []string{output1},
			wantError:  true,
		},
		{
			name: "wrap with error in second function",
			callFuncs: []Func{
				fakeCallFunc(t, false, output1),
				fakeCallFunc(t, true, output2),
			},
			repo:       testRepo,
			wantOutput: []string{output1, output2},
			wantError:  true,
		},
		{
			name:       "wrap with no CallFuncs",
			callFuncs:  []Func{},
			repo:       testRepo,
			wantOutput: []string{},
		},
		{
			name: "wrap with single function",
			callFuncs: []Func{
				fakeCallFunc(t, false, "test output"),
			},
			repo:       testRepo,
			wantOutput: []string{"test output"},
		},
		{
			name: "wrap repository cloning scenario",
			callFuncs: []Func{
				fakeCallFunc(t, false, "test after clone"),
			},
			repo:       "nonexistent-repo",
			wantOutput: []string{"test after clone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := Wrap(tt.callFuncs...)
			if wrapper == nil {
				t.Fatal("Wrap returned nil")
			}

			ch := output.NewChannel(ctx, tt.repo, nil, nil)
			err := wrapper(ctx, ch)
			ch.Close()

			// Collect output - reconstruct lines from byte chunks
			var buffer []byte
			for msg := range ch.Out() {
				buffer = append(buffer, msg...)
			}
			res := strings.Split(strings.TrimSuffix(string(buffer), "\n"), "\n")
			if len(buffer) == 0 {
				res = []string{}
			}

			testhelper.AssertOutput(t, res, tt.wantOutput, err, tt.wantError)
		})
	}
}

// TestExec tests the Exec function which creates CallFuncs for shell commands
func TestExec(t *testing.T) {
	ctx := loadFixture(t)

	testMessage := "test message"
	multilineOutput := "line1\nline2\nline3"

	tests := []struct {
		name       string
		command    string
		args       []string
		wantOutput []string
		wantError  bool
	}{
		{
			name:       "basic echo command",
			command:    "echo",
			args:       []string{testMessage},
			wantOutput: []string{testMessage},
		},
		{
			name:       "multiple arguments",
			command:    "echo",
			args:       []string{"hello", "world", "test"},
			wantOutput: []string{"hello world test"},
		},
		{
			name:      "nonexistent command",
			command:   "nonexistent-command-xyz",
			args:      []string{"arg1"},
			wantError: true,
		},
		{
			name:      "empty command",
			command:   "",
			args:      []string{"arg1"},
			wantError: true,
		},
		{
			name:       "multiline output",
			command:    "echo",
			args:       []string{multilineOutput},
			wantOutput: []string{"line1", "line2", "line3"},
		},
		{
			name:      "command with exit failure",
			command:   "false",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create repository directory for commands to run in
			repoPath := utils.RepoPath(ctx, "test-repo")
			if err := os.MkdirAll(repoPath, 0755); err != nil {
				t.Fatalf("Failed to create repo directory: %v", err)
			}
			t.Cleanup(func() {
				os.RemoveAll(repoPath)
			})

			execFunc := Exec(tt.command, tt.args...)
			if execFunc == nil {
				t.Fatal("Exec returned nil")
			}

			ch := output.NewChannel(ctx, "test-repo", nil, nil)
			err := execFunc(ctx, ch)
			ch.Close()

			// Collect output - reconstruct lines from byte chunks
			var buffer []byte
			for msg := range ch.Out() {
				buffer = append(buffer, msg...)
			}
			output := strings.Split(strings.TrimSuffix(string(buffer), "\n"), "\n")
			if len(buffer) == 0 {
				output = []string{}
			}

			testhelper.AssertOutput(t, output, tt.wantOutput, err, tt.wantError)
		})
	}
}
