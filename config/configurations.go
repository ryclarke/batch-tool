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
	CfgFile string

	// Version is dynamically set at build time using the -X linker flag.
	// Default value is used for testing and development builds.
	Version = "dev"
)

const (
	EnvGopath = "gopath"

	GitUser      = "git.user"
	GitHost      = "git.host"
	GitProject   = "git.project"
	GitProvider  = "git.provider"
	GitDirectory = "git.directory"
	SourceBranch = "git.default-branch"

	// User, Host, Project, Repo
	CloneSSHURLTmpl = "ssh://%s@%s/%s/%s.git"

	SortRepos        = "repos.sort"
	RepoAliases      = "repos.aliases"
	UnwantedLabels   = "repos.unwanted-labels"
	SkipUnwanted     = "repos.skip-unwanted"
	SuperSetLabel    = "repos.catch-all"
	DefaultReviewers = "repos.reviewers"
	CatalogCacheFile = "repos.cache.filename"
	CatalogCacheTTL  = "repos.cache.ttl"

	CommitAmend   = "commit.amend"
	CommitMessage = "commit.message"

	Branch    = "branch"
	Reviewers = "reviewers"
	AuthToken = "auth-token"

	TokenLabel  = "repos.tokens.label"
	TokenSkip   = "repos.tokens.skip"
	TokenForced = "repos.tokens.forced"

	ChannelBuffer  = "channels.buffer-size"
	MaxConcurrency = "channels.max-concurrency"
	WriteBackoff   = "channels.write-backoff"

	GithubHourlyWriteLimit = "github.hourly-write-limit"
	GithubBackoffSmall     = "github.write-backoff-small"
	GithubBackoffLarge     = "github.write-backoff-large"
)

// Init reads in config file and ENV variables if set.
func Init() {
	initialize()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		viper.SetConfigName("batch-tool")

		// Search in the working directory
		viper.AddConfigPath(".")

		// Search in the user's config directory
		if usrConfig, err := os.UserConfigDir(); err == nil {
			viper.AddConfigPath(usrConfig)
		}

		// On Darwin, os.UserConfigDir() returns ~/Library/Application Support.  As this is to be used from
		// the command line, it's more likely that the user will want to use XDG_CONFIG_HOME instead.
		if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
			viper.AddConfigPath(xdgConfigHome)
		} else if homeDir, err := os.UserHomeDir(); err == nil {
			viper.AddConfigPath(filepath.Join(homeDir, ".config"))
		}

		// Search in the executable's directory
		if ex, err := os.Executable(); err == nil {
			viper.AddConfigPath(filepath.Dir(ex))
		}
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Printf("Using config file: %v\n\n", viper.ConfigFileUsed())
	}
}

func initialize() {
	// Default user for SSH clone.
	viper.SetDefault(GitUser, "git")

	viper.SetDefault(GitHost, "github.com")
	viper.SetDefault(GitProvider, "github")
	viper.SetDefault(SourceBranch, "main")
	viper.SetDefault(SortRepos, true)

	viper.SetDefault(SkipUnwanted, true)
	viper.SetDefault(UnwantedLabels, []string{"deprecated", "poc"})
	viper.SetDefault(SuperSetLabel, "all")

	viper.SetDefault(CatalogCacheFile, ".catalog")
	viper.SetDefault(CatalogCacheTTL, "24h")

	viper.SetDefault(ChannelBuffer, 100)
	viper.SetDefault(MaxConcurrency, runtime.NumCPU()) // Default to number of logical CPUs
	viper.SetDefault(WriteBackoff, "1s")

	// GitHub's secondary rate limit is 80 requests per minute, or 500 requests per hour
	// 1s keeps us safely under the per-minute limit
	// 8s keeps us safely under the per-hour limit when working with many repositories
	viper.SetDefault(GithubHourlyWriteLimit, 500)
	viper.SetDefault(GithubBackoffSmall, "1s")
	viper.SetDefault(GithubBackoffLarge, "8s")

	// default reviewers in the form `repo: [reviewers...]`
	viper.SetDefault(DefaultReviewers, map[string][]string{})

	// aliases in the form `alias: [repos...]`
	viper.SetDefault(RepoAliases, map[string][]string{})

	// default git directory is $GOPATH/src if GOPATH is set, or current working directory otherwise
	viper.SetDefault(GitDirectory, defaultGitdir())

	// defaults for token identifiers
	viper.SetDefault(TokenLabel, "~")
	viper.SetDefault(TokenSkip, "!")
	viper.SetDefault(TokenForced, "+")
}

// LoadFixture will load example configuration; for testing only!
func LoadFixture(dir string) error {
	viper.Reset()
	initialize()

	viper.SetConfigName("example-config")
	viper.AddConfigPath(dir)

	return viper.ReadInConfig()
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
