package output

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// ansiPattern matches CSI and OSC escape sequences, such as the colors that
// `git status` is told to always emit for the TUI.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// isTerminal reports whether w writes to an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	fd := f.Fd()

	return fd <= math.MaxInt && term.IsTerminal(int(fd)) //nolint:gosec // bounds checked above
}

// NativeHandler is a plain output Handler for scripts, CI, agents, and any terminal.
// Repository output is streamed to stdout in repository order under a header per
// repository. Errors, the command line, and a closing summary go to stderr. Escape
// sequences are stripped unless stdout is a terminal.
func NativeHandler(cmd *cobra.Command, channels []Channel) {
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()

	if len(channels) == 0 {
		fmt.Fprintln(errOut, noReposText)
		return
	}

	ctx := cmd.Context()
	plain := !isTerminal(out)
	start := time.Now()

	fmt.Fprintln(errOut, buildCommandString(cmd))

	// Drain every channel as soon as it produces output. Printing in repository order
	// must not stop a later repository from finishing: a worker blocked on a full
	// buffer would keep its concurrency slot, and the repository being printed might
	// never get one.
	results := make([]*nativeResult, len(channels))
	for i, ch := range channels {
		results[i] = collectNative(ch)
	}

	failed := make([]string, 0)

	for i, ch := range channels {
		name := utils.DisplayRepo(ctx, ch.Name())
		fmt.Fprintf(out, "\n------ %s ------\n", name)

		errs := results[i].stream(func(data []byte) {
			if plain {
				data = ansiPattern.ReplaceAll(data, nil)
			}

			out.Write(data) //nolint:errcheck // nothing useful to do if stdout is gone
		})

		for _, err := range errs {
			fmt.Fprintf(errOut, "ERROR: %s: %v\n", name, err)
		}

		if len(errs) > 0 {
			failed = append(failed, name)
		}
	}

	elapsed := time.Since(start).Round(time.Second)

	fmt.Fprintln(errOut)

	if len(failed) > 0 {
		fmt.Fprintf(errOut, summaryTextFail+"\n", len(channels), len(failed), elapsed)
		fmt.Fprintf(errOut, "Failed: %s\n", strings.Join(failed, ", "))

		return
	}

	fmt.Fprintf(errOut, summaryText+"\n", len(channels), elapsed)
}

// nativeResult accumulates one channel's output and errors while earlier
// repositories are still being printed.
type nativeResult struct {
	mu     sync.Mutex
	cond   *sync.Cond
	chunks [][]byte
	errs   []error
	done   bool
}

// collectNative starts draining ch in the background and returns its result.
func collectNative(ch Channel) *nativeResult {
	r := &nativeResult{}
	r.cond = sync.NewCond(&r.mu)

	go func() {
		out, errs := ch.Out(), ch.Err()

		for out != nil || errs != nil {
			select {
			case data, ok := <-out:
				if !ok {
					out = nil
					continue
				}

				r.mu.Lock()
				r.chunks = append(r.chunks, data)
				r.mu.Unlock()
				r.cond.Broadcast()

			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}

				r.mu.Lock()
				r.errs = append(r.errs, err)
				r.mu.Unlock()
			}
		}

		r.mu.Lock()
		r.done = true
		r.mu.Unlock()
		r.cond.Broadcast()
	}()

	return r
}

// stream passes each chunk to write as it arrives, and returns the channel's
// errors once both of its streams have closed.
func (r *nativeResult) stream(write func([]byte)) []error {
	next := 0

	for {
		r.mu.Lock()
		for next == len(r.chunks) && !r.done {
			r.cond.Wait()
		}

		pending, done := r.chunks[next:], r.done
		next = len(r.chunks)
		r.mu.Unlock()

		for _, data := range pending {
			write(data)
		}

		if done {
			r.mu.Lock()
			defer r.mu.Unlock()

			r.chunks = nil

			return r.errs
		}
	}
}

// NativeCatalog displays the repository catalog in a simple text format.
func NativeCatalog(cmd *cobra.Command) {
	ctx := cmd.Context()
	viper := config.Viper(ctx)
	out := cmd.OutOrStdout()

	repoNames := make([]string, 0, len(catalog.Catalog))
	for name := range catalog.Catalog {
		repoNames = append(repoNames, name)
	}
	sort.Strings(repoNames)

	fmt.Fprintf(out, "Repository Catalog (%d repositories)\n", len(catalog.Catalog))
	fmt.Fprintln(out)

	for _, name := range repoNames {
		repo := catalog.Catalog[name]

		fmt.Fprintf(out, "## %s\n", utils.DisplayRepo(ctx, name))

		if repo.Description != "" {
			fmt.Fprintf(out, "   %s\n", repo.Description)
		}

		if len(repo.Labels) > 0 {
			labels := slices.Clone(repo.Labels)
			if viper.GetBool(config.SortRepos) {
				sort.Strings(labels)
			}

			fmt.Fprintf(out, "   Labels: %s\n", strings.Join(labels, ", "))
		}

		visibility := "private"
		if repo.Public {
			visibility = "public"
		}

		if repo.Archived {
			visibility += " (archived)"
		}

		fmt.Fprintf(out, "   Project: %s | Default Branch: %s | Visibility: %s\n", repo.Project, repo.DefaultBranch, visibility)
		fmt.Fprintln(out)
	}
}

// NativeLabels prints labels in the native/terminal output format.
// When no filters are provided, it prints all available labels and their repositories,
// hiding unwanted labels unless verbose is set. When filters are provided, it prints a
// set-theory representation of the filter and the matched repositories.
func NativeLabels(cmd *cobra.Command, verbose bool, filters ...string) {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	if len(filters) > 0 {
		labelGroup, repos := catalog.ParseLabels(ctx, filters...)
		printSet(cmd, verbose, labelGroup, displayRepos(ctx, repos))

		return
	}

	fmt.Fprintln(out, "Available labels:")

	unwantedRepos := getUnwantedRepos(ctx)
	for _, label := range listLabels(ctx, verbose) {
		printLabel(ctx, out, label, unwantedRepos)
	}
}

// printLabel prints a label with its repository count and repositories. Repositories
// carrying an unwanted label are marked, unless the label itself is unwanted.
func printLabel(ctx context.Context, out io.Writer, label labelWithRepos, unwantedRepos mapset.Set[string]) {
	name := utils.CleanFilter(ctx, label.name)

	marker := ""
	if label.isUnwanted {
		marker = " (unwanted)"
	}

	if label.empty {
		fmt.Fprintf(out, "  ~ %s ~ %s%s\n", name, emptyLabelText, marker)
		return
	}

	fmt.Fprintf(out, "  ~ %s ~ %s%s\n", name, labelCountText(label, unwantedRepos), marker)

	repos := make([]string, len(label.repos))
	for i, repo := range label.repos {
		repos[i] = utils.DisplayRepo(ctx, repo)
		if !label.isUnwanted && unwantedRepos.Contains(repo) {
			repos[i] += " (unwanted)"
		}
	}

	fmt.Fprintln(out, strings.Join(repos, ", "))
}

// printSet prints a set-theory representation of the provided filters in native format.
func printSet(cmd *cobra.Command, verbose bool, labelGroup catalog.LabelGroup, repos []string) {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "You've selected the following set:\n%s\n\n", labelGroup.String())

	switch n := len(repos); n {
	case 0:
		fmt.Fprintln(out, "This matches no known repositories")
	case 1:
		fmt.Fprintf(out, "This matches 1 repository: %s\n", repos[0])
	default:
		fmt.Fprintf(out, "This matches %d repositories, listed below:\n%s\n", n, strings.Join(repos, ", "))
	}

	if !verbose {
		return
	}

	forced, included, excluded := labelGroup.ToSlices()
	unwantedRepos := getUnwantedRepos(ctx)

	for _, section := range []struct {
		kind  string
		names []string
	}{
		{"Forced", forced},
		{"Included", included},
		{"Excluded", excluded},
	} {
		labels := labelEntries(ctx, buildLabelWithRepos(ctx, section.names))
		if len(labels) == 0 {
			continue
		}

		fmt.Fprintf(out, "\n%s labels:\n", section.kind)

		for _, label := range labels {
			printLabel(ctx, out, label, unwantedRepos)
		}
	}
}
