package call

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"

	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

// Do executes the provided Wrapper on each repository, operating
// asynchronously by default with configurable concurrency limits.
// Repository aliases are also expanded here to allow for configurable repository grouping.
func Do(repos []string, w io.Writer, fwrap Wrapper) {
	repos = processArguments(repos)

	// initialize channel set
	ch := make([]chan string, len(repos))
	for i := range repos {
		ch[i] = make(chan string, viper.GetInt(config.ChannelBuffer))
	}

	if viper.GetBool(config.UseSync) {
		// execute workers and print output synchronously
		for i, repo := range repos {
			go fwrap(repo, ch[i])

			for msg := range ch[i] {
				fmt.Fprintln(w, msg)
			}
		}

		return
	}

	// Async execution with concurrency limiting
	maxConcurrency := viper.GetInt(config.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU() // fallback to number of logical CPUs
	}

	// Use a semaphore (buffered channel) to limit concurrency
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// start workers with concurrency limit
	for i, repo := range repos {
		wg.Add(1)

		go func(index int, repoName string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

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

// DoAsync always operates asynchronously regardless of configuration
func DoAsync(repos []string, w io.Writer, fwrap Wrapper) {
	viper.Set(config.UseSync, false)
	Do(repos, w, fwrap)
}

// DoSync always operates synchronously regardless of configuration
func DoSync(repos []string, w io.Writer, fwrap Wrapper) {
	viper.Set(config.UseSync, true)
	Do(repos, w, fwrap)
}

func processArguments(args []string) []string {
	repos := catalog.RepositoryList(args...).ToSlice()

	// Sort the repositories alphabetically
	if viper.GetBool(config.SortRepos) {
		sort.Strings(repos)
	}

	return repos
}
