package call

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

// TestOrderedOutput tests that OrderedOutput properly handles and prints messages and errors
func TestOrderedOutput(t *testing.T) {
	ctx := loadFixture(t)
	setupDirs(t, ctx, []string{"repo1", "repo2"})

	viper := config.Viper(ctx)
	viper.Set(config.MaxConcurrency, 2)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// CallFunc that returns an error
	errorFunc := func(_ context.Context, repo string, ch chan<- string) error {
		ch <- "some output before error"
		return errors.New("test error for " + repo)
	}

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	Do(cmd, []string{"repo1", "repo2"}, errorFunc)

	output := buf.String()
	errOutput := errBuf.String()

	// Verify headers and output were printed
	checkOutputContains(t, output, []string{"------ repo1 ------", "------ repo2 ------", "some output before error"})

	// Verify errors were printed to stderr
	checkOutputContains(t, errOutput, []string{"ERROR:", "test error for repo1", "test error for repo2"})
}
