package utils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
)

// ValidateRequiredConfig checks viper and returns an error if a key isn't set
func ValidateRequiredConfig(opts ...string) error {
	for _, opt := range opts {
		if viper.GetString(opt) == "" {
			return fmt.Errorf("%s is required - set as flag or env", opt)
		}
	}

	return nil
}

// LookupReviewers returns the list of reviewers for the given repository
func LookupReviewers(name string) []string {
	// Use the provided list of reviewers
	if revs := viper.GetStringSlice(config.Reviewers); len(revs) > 0 {
		return revs
	}

	// Use default reviewers for the given repository
	return viper.GetStringMapStringSlice(config.DefaultReviewers)[name]
}

// ParseRepo splits a repo identifier into its component parts
func ParseRepo(repo string) (host, project, name string) {
	parts := strings.Split(strings.Trim(repo, "/ "), "/")
	name = parts[len(parts)-1]

	if len(parts) > 1 {
		project = parts[len(parts)-2]
	} else {
		project = viper.GetString(config.GitProject)
	}

	if len(parts) > 2 {
		host = strings.Join(parts[:len(parts)-3], "/")
	} else {
		host = viper.GetString(config.GitHost)
	}

	return
}

// RepoPath returns the full repository path for the given name
func RepoPath(repo string) string {
	host, project, name := ParseRepo(repo)

	return filepath.Join(
		viper.GetString(config.EnvGopath),
		"src", host, project, name,
	)
}

// RepoURL returns the repository remote url for the given name
func RepoURL(repo string) string {
	host, project, name := ParseRepo(repo)

	return fmt.Sprintf(config.CloneSSHURLTmpl,
		viper.GetString(config.GitUser),
		host, project, name,
	)
}

// LookupBranch returns the target branch for the given repository
func LookupBranch(name string) (string, error) {
	branch := viper.GetString(config.Branch)
	if branch == "" {
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = RepoPath(name)

		output, err := cmd.Output()
		if err != nil {
			return "", err
		}

		branch = strings.TrimSpace(string(output))
		viper.Set(config.Branch, branch)
	}

	return branch, nil
}

// ValidateBranch returns an error if the current git branch is the source branch
func ValidateBranch(repo string, ch chan<- string) error {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = RepoPath(repo)

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(output)) == strings.TrimSpace(viper.GetString(config.SourceBranch)) {
		return fmt.Errorf("skipping operation - %s is the source branch", output)
	}

	return nil
}
