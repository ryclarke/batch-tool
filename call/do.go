package call

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"

	"github.com/spf13/viper"
	"golang.org/x/sync/semaphore"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

// Do executes the provided Wrapper on each repository, operating
// asynchronously by default with configurable concurrency limits.
// Repository aliases are also expanded here to allow for configurable repository grouping.
func Do(repos []string, w io.Writer, fwrap Wrapper) {
	ctx := context.Background()
	repos = processArguments(repos)

	// initialize channel set
	ch := make([]chan string, len(repos))
	for i := range repos {
		ch[i] = make(chan string, viper.GetInt(config.ChannelBuffer))
	}

	// Determine concurrency level
	maxConcurrency := viper.GetInt(config.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU() // fallback to number of logical CPUs
	}

	sem := semaphore.NewWeighted(int64(maxConcurrency))

	var wg sync.WaitGroup

	// start workers with concurrency limit
	for i, repo := range repos {
		wg.Add(1)

		go func(index int, repoName string) {
			defer wg.Done()

			// Acquire semaphore
			if err := sem.Acquire(ctx, 1); err != nil {
				// Context cancelled, just return
				return
			}
			defer sem.Release(1) // Release semaphore

			// Execute the wrapper
			fwrap(repoName, ch[index])
		}(i, repo)
	}

	// Wait for all workers to complete, then close channels
	go func() {
		wg.Wait()
		// All workers are done, channels should already be closed by wrappers
	}()

	// batch and print ordered output
	for i := range repos {
		for msg := range ch[i] {
			fmt.Fprintln(w, msg)
		}
	}
}

func processArguments(args []string) []string {
	repos := catalog.RepositoryList(args...).ToSlice()

	// Sort the repositories alphabetically
	if viper.GetBool(config.SortRepos) {
		sort.Strings(repos)
	}

	// Determine appropriate write backoff based on number of repositories to be processed (GitHub only)
	if viper.GetString(config.GitProvider) == "github" && len(repos) < viper.GetInt(config.GithubHourlyWriteLimit)/2 {
		viper.Set(config.WriteBackoff, viper.GetString(config.GithubBackoffSmall))
	} else {
		viper.Set(config.WriteBackoff, viper.GetString(config.GithubBackoffLarge))
	}

	return repos
}
