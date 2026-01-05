package call

import (
	"context"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

// Do executes the provided Func on each repository, operating asynchronously by default with configurable
// concurrency limits. Repository aliases are also expanded here to allow for configurable repository grouping.
// Output formatting can be fully customized by optionally providing one or more OutputHandler functions. Each
// repository will also be cloned first if it is missing from the local file system.
func Do(cmd *cobra.Command, repos []string, callFunc Func, handler ...output.Handler) {
	ctx := cmd.Context()
	viper := config.Viper(ctx)
	repos = processArguments(ctx, repos)

	// Determine concurrency level
	maxConcurrency := viper.GetInt(config.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU() // fallback to number of logical CPUs
	}

	sem := semaphore.NewWeighted(int64(maxConcurrency))
	wg := new(sync.WaitGroup)

	// initialize channel set for Func output
	channels := make([]output.Channel, len(repos))
	for i := range repos {
		channels[i] = output.NewChannel(ctx, repos[i], sem, wg)
	}

	// start workers with concurrency limit
	for i := range repos {
		wg.Add(1)
		go runCallFunc(ctx, channels[i], callFunc)
	}

	// use the default output handler if none provided
	if len(handler) == 0 {
		handler = append(handler, output.GetHandler(ctx))
	}

	// process output using provided handler(s)
	for _, handle := range handler {
		handle(cmd, channels)
	}

	wg.Wait()
}

// runCallFunc executes the provided Func for a single repository, managing concurrency via the provided semaphore and wait group.
// Output channels are closed after execution, and the repository is cloned first if it does not exist locally.
func runCallFunc(ctx context.Context, ch output.Channel, callFunc Func) {
	defer ch.Close()

	if err := ch.Start(1); err != nil {
		ch.WriteError(err)
		return
	}

	// Initial empty line to signal start of Func execution
	ch.WriteString("")

	// If the repository is missing, attempt to clone it first
	repoDir := utils.RepoPath(ctx, ch.Name())
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		// Create the directory if it doesn't exist yet
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			ch.WriteError(err)
			return
		}

		// Execute git clone into the target directory
		if err := Exec("git", "clone", utils.RepoURL(ctx, ch.Name()), repoDir)(ctx, ch); err != nil {
			// Clone failed, return the error and abort further processing
			ch.WriteError(err)
			return
		}
	}

	// Execute the provided Func for the repository
	if err := callFunc(ctx, ch); err != nil {
		ch.WriteError(err)
	}
}

// processArguments expands repository aliases, sorts repositories if configured, and sets appropriate write backoff.
func processArguments(ctx context.Context, args []string) []string {
	viper := config.Viper(ctx)
	repos := catalog.RepositoryList(ctx, args...).ToSlice()

	// Sort the repositories alphabetically
	if viper.GetBool(config.SortRepos) {
		sort.Strings(repos)
	}

	// Determine appropriate write backoff based on number of repositories to be processed (within provider-specific limits)
	switch viper.GetString(config.GitProvider) {
	case "github":
		if len(repos) < viper.GetInt(config.GithubHourlyWriteLimit)/2 {
			viper.Set(config.WriteBackoff, viper.GetString(config.GithubBackoffSmall))
		} else {
			viper.Set(config.WriteBackoff, viper.GetString(config.GithubBackoffLarge))
		}
	case "bitbucket":
		// use default write backoff
	}

	return repos
}
