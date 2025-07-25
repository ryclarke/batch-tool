package call

import (
	"fmt"
	"os"

	"github.com/ryclarke/batch-tool/utils"
)

// Wrapper defines all work to be performed on a repository. A Wrapper
// must close the output channel to signal that all work is completed.
type Wrapper func(repo string, ch chan<- string)

// Wrap each provided CallFunc into a new Wrapper that executes them in
// the provided order before closing the channel and terminating. Wrap
// will also attempt to clone the repository first if it is missing.
func Wrap(calls ...CallFunc) Wrapper {
	return func(repo string, ch chan<- string) {
		defer func() {
			close(ch)
		}()

		ch <- fmt.Sprintf("------ %s ------", repo)

		// if the repository is missing, attempt to clone it first
		if _, err := os.Stat(utils.RepoPath(repo)); os.IsNotExist(err) {
			ch <- "Repository not found, cloning...\n"

			if err = Exec("git", "clone", "--progress", utils.RepoURL(repo))("", ch); err != nil {
				ch <- fmt.Sprintln("ERROR:", err)

				return
			}
		}

		// execute each CallFunc, stopping if an error is encountered
		for _, call := range calls {
			if err := call(repo, ch); err != nil {
				ch <- fmt.Sprintln("ERROR:", err)

				return
			}
		}

		ch <- ""
	}
}
