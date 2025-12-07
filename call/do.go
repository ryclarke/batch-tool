package call

import (
	"context"
	"os"
	"runtime"
	"sort"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

// Do executes the provided CallFunc on each repository, operating asynchronously by default with configurable
// concurrency limits. Repository aliases are also expanded here to allow for configurable repository grouping.
// Output formatting can be fully customized by optionally providing one or more OutputHandler functions. Each
// repository will also be cloned first if it is missing from the local file system.
func Do(cmd *cobra.Command, repos []string, callFunc CallFunc, handler ...OutputHandler) {
	ctx := cmd.Context()
	viper := config.Viper(ctx)
	repos = processArguments(ctx, repos)

	// initialize channel set for CallFunc output
	output := make([]chan string, len(repos))
	errs := make([]chan error, len(repos))
	for i := range repos {
		output[i] = make(chan string, viper.GetInt(config.ChannelBuffer))
		errs[i] = make(chan error, 1)
	}

	// Determine concurrency level
	maxConcurrency := viper.GetInt(config.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU() // fallback to number of logical CPUs
	}

	sem := semaphore.NewWeighted(int64(maxConcurrency))
	wg := new(sync.WaitGroup)

	// start workers with concurrency limit
	for i := range repos {
		wg.Add(1)
		go runCallFunc(ctx, sem, wg, callFunc, repos[i], output[i], errs[i])
	}

	// use the default output handler if none provided
	if len(handler) == 0 {
		handler = append(handler, OrderedOutput)
	}

	// process output using provided handler(s)
	for _, handle := range handler {
		handle(cmd, repos, readOnlyChan(output), readOnlyChan(errs))
	}

	wg.Wait()
}

// runCallFunc executes the provided CallFunc for a single repository, managing concurrency via the provided semaphore and wait group.
// Output channels are closed after execution, and the repository is cloned first if it does not exist locally.
func runCallFunc(ctx context.Context, sem *semaphore.Weighted, wg *sync.WaitGroup, callFunc CallFunc, repoName string, ch chan<- string, er chan<- error) {
	defer func() {
		// signal worker completion and close channels
		wg.Done()
		close(ch)
		close(er)
	}()

	// Acquire semaphore
	if err := sem.Acquire(ctx, 1); err != nil {
		// Context cancelled, return the error and abort further processing
		er <- err
		return
	}
	defer sem.Release(1) // Release semaphore

	// If the repository is missing, attempt to clone it first
	if _, err := os.Stat(utils.RepoPath(ctx, repoName)); os.IsNotExist(err) {
		ch <- "Repository not found, cloning...\n"

		if err = Exec("git", "clone", "--progress", utils.RepoURL(ctx, repoName))(ctx, "", ch); err != nil {
			// Clone failed, return the error and abort further processing
			er <- err
			return
		}
	}

	// Execute the provided CallFunc for the repository
	if err := callFunc(ctx, repoName, ch); err != nil {
		er <- err
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

// readOnlyChan converts a slice of bidirectional channels to a slice of read-only channels.
func readOnlyChan[T any](chans []chan T) []<-chan T {
	roCh := make([]<-chan T, len(chans))
	for i, ch := range chans {
		roCh[i] = ch
	}
	return roCh
}
