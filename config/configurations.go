// Package config provides configuration management for batch-tool.
package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var (
	// CfgFile specifies the configuration file path
	CfgFile string

	// Version is dynamically set at build time using the -X linker flag.
	// Default value is used for testing and development builds.
	Version = "dev"
)

const (
	EnvGopath = "gopath"

	GitUser       = "git.user"
	GitHost       = "git.host"
	GitProject    = "git.project"
	GitProjects   = "git.projects"
	GitProvider   = "git.provider"
	GitDirectory  = "git.directory"
	DefaultBranch = "git.default-branch"
	StashUpdates  = "git.stash-updates"

	// CloneSSHURLTmpl is the SSH URL template with placeholders: User, Host, Project, Repo
	CloneSSHURLTmpl = "ssh://%s@%s/%s/%s.git"

	SortRepos        = "repos.sort"
	RepoAliases      = "repos.aliases"
	UnwantedLabels   = "repos.unwanted-labels"
	SkipUnwanted     = "repos.skip-unwanted"
	SuperSetLabel    = "repos.catch-all"
	DefaultReviewers = "repos.reviewers"
	CatalogCachePath = "repos.cache.path"
	CatalogCacheTTL  = "repos.cache.ttl"

	Branch    = "branch"
	AuthToken = "auth-token"

	TokenLabel  = "repos.tokens.label"
	TokenSkip   = "repos.tokens.skip"
	TokenForced = "repos.tokens.forced"

	OutputStyle  = "channels.output-style"
	PrintResults = "channels.print-results"
	WaitOnExit   = "channels.wait-on-exit"

	ChannelBuffer  = "channels.buffer-size"
	MaxConcurrency = "channels.max-concurrency"
	WriteBackoff   = "channels.write-backoff"

	GithubHourlyWriteLimit = "github.hourly-write-limit"
	GithubBackoffSmall     = "github.write-backoff-small"
	GithubBackoffLarge     = "github.write-backoff-large"

	// == COMMAND FLAGS == //
	CmdEnv = "cmd.args.env"

	// git
	GitCommitMessage = "git.args.commit.message"
	GitCommitAmend   = "git.args.commit.amend"
	GitCommitPush    = "git.args.commit.push"
	GitStashAllowAny = "git.args.stash.allow-any"

	// pr
	PrOptions        = "pr.args.options"
	PrTitle          = "pr.args.title"
	PrDescription    = "pr.args.description"
	PrDraft          = "pr.args.draft"
	PrReviewers      = "pr.args.reviewers"
	PrTeamReviewers  = "pr.args.team-reviewers"
	PrResetReviewers = "pr.args.reset-reviewers"
	PrAllReviewers   = "pr.args.all-reviewers"
	PrBaseBranch     = "pr.args.base-branch"
	PrForceMerge     = "pr.args.force-merge"

	// make
	MakeTargets = "make.args.targets"
)

// Init reads in config file and ENV variables if set.
func Init(ctx context.Context) context.Context {
	v := New()

	if CfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(CfgFile)
	} else {
		v.SetConfigName("batch-tool")

		// Search in the working directory
		v.AddConfigPath(".")

		// Search in the user's config directory
		if usrConfig, err := os.UserConfigDir(); err == nil {
			v.AddConfigPath(usrConfig)
		}

		// On Darwin, os.UserConfigDir() returns ~/Library/Application Support.  As this is to be used from
		// the command line, it's more likely that the user will want to use XDG_CONFIG_HOME instead.
		if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
			v.AddConfigPath(xdgConfigHome)
		} else if homeDir, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(homeDir, ".config"))
		}

		// Search in the executable's directory
		if ex, err := os.Executable(); err == nil {
			v.AddConfigPath(filepath.Dir(ex))
		}
	}

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "Using config file: %v\n\n", v.ConfigFileUsed())
	}

	return SetViper(ctx, v)
}

// New creates a new Viper instance with default configuration.
func New() *viper.Viper {
	v := viper.NewWithOptions(viper.EnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_")))
	v.AutomaticEnv() // read in environment variables that match
	setDefaults(v)

	return v
}

func setDefaults(v *viper.Viper) {
	// Default user for SSH clone.
	v.SetDefault(GitUser, "git")

	v.SetDefault(GitHost, "github.com")
	v.SetDefault(GitProvider, "github")
	v.SetDefault(GitProjects, []string{})
	v.SetDefault(DefaultBranch, "main")
	v.SetDefault(StashUpdates, false)
	v.SetDefault(SortRepos, true)

	v.SetDefault(SkipUnwanted, true)
	v.SetDefault(UnwantedLabels, []string{"deprecated", "poc"})
	v.SetDefault(SuperSetLabel, "all")

	v.SetDefault(CatalogCachePath, "") // empty means use default: gitdir/host/.batch-tool-cache.json
	v.SetDefault(CatalogCacheTTL, "24h")
	v.SetDefault(OutputStyle, "tui")
	v.SetDefault(WaitOnExit, true) // Wait for user input after completion by default
	v.SetDefault(ChannelBuffer, 100)
	v.SetDefault(MaxConcurrency, runtime.NumCPU()) // Default to number of logical CPUs
	v.SetDefault(WriteBackoff, "1s")

	// GitHub's secondary rate limit is 80 requests per minute, or 500 requests per hour
	// 1s keeps us safely under the per-minute limit
	// 8s keeps us safely under the per-hour limit when working with many repositories
	v.SetDefault(GithubHourlyWriteLimit, 500)
	v.SetDefault(GithubBackoffSmall, "1s")
	v.SetDefault(GithubBackoffLarge, "8s")

	// default reviewers in the form `repo: [reviewers...]`
	v.SetDefault(DefaultReviewers, map[string][]string{})

	// aliases in the form `alias: [repos...]`
	v.SetDefault(RepoAliases, map[string][]string{})

	// default git directory is $GOPATH/src if GOPATH is set, or current working directory otherwise
	v.SetDefault(GitDirectory, defaultGitdir())

	// defaults for token identifiers
	v.SetDefault(TokenLabel, "~")
	v.SetDefault(TokenSkip, "!")
	v.SetDefault(TokenForced, "+")
}

func defaultGitdir() string {
	dir := os.Getenv("GOPATH")
	if dir == "" {
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()

		cmd := exec.CommandContext(ctx, "go", "env", "GOPATH")

		// If GOPATH is not set, try to get it from the golang CLI.
		if path, err := cmd.Output(); err == nil {
			dir = strings.TrimSpace(string(path))
		} else {
			// If that fails, use the current working directory.
			dir, err = os.Getwd()
			if err != nil {
				panic(fmt.Sprintf("Failed to determine current working directory: %v", err))
			}

			return dir
		}
	}

	return filepath.Join(dir, "src")
}
