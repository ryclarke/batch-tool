package utils

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryclarke/batch-tool/config"
)

// functions initialized in catalog package to avoid circular import
var (
	// CatalogProjectLookup is a function type for looking up project from catalog
	CatalogProjectLookup = defaultProjectLookup
	// CatalogBranchLookup is a function type for looking up branch from catalog
	CatalogBranchLookup = defaultBranchLookup
)

// ParseRepo splits a repo identifier into its component parts
func ParseRepo(ctx context.Context, repo string) (host, project, name string) {
	viper := config.Viper(ctx)

	parts := strings.Split(strings.Trim(repo, "/ "), "/")
	name = parts[len(parts)-1]

	if len(parts) > 1 {
		project = parts[len(parts)-2]
	} else {
		project = CatalogProjectLookup(ctx, name)
	}

	if len(parts) > 2 {
		host = strings.Join(parts[:len(parts)-3], "/")
	} else {
		host = viper.GetString(config.GitHost)
	}

	return
}

// RepoPath returns the full repository path for the given name
func RepoPath(ctx context.Context, repo string) string {
	viper := config.Viper(ctx)

	// If repo is empty, return the base working directory itself (for operations like cloning)
	if repo == "" {
		path, err := filepath.Abs(viper.GetString(config.GitDirectory))
		if err != nil {
			panic(fmt.Sprintf("error determining absolute working directory path: %v", err))
		}

		return path
	}

	host, project, name := ParseRepo(ctx, repo)

	path, err := filepath.Abs(filepath.Join(viper.GetString(config.GitDirectory), host, project, name))
	if err != nil {
		panic(fmt.Sprintf("error determining absolute repo path: %v", err))
	}

	return path
}

// RepoURL returns the repository remote url for the given name
func RepoURL(ctx context.Context, repo string) string {
	viper := config.Viper(ctx)

	host, project, name := ParseRepo(ctx, repo)

	return fmt.Sprintf(config.CloneSSHURLTmpl,
		viper.GetString(config.GitUser),
		host, project, name,
	)
}

// LookupBranch returns the target branch for the given repository
func LookupBranch(ctx context.Context, name string) (string, error) {
	viper := config.Viper(ctx)

	branch := viper.GetString(config.Branch)
	if branch == "" {
		// don't use Cmd helper because of infinite recursion
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = RepoPath(ctx, name)

		output, err := cmd.Output()
		if err != nil {
			return "", err
		}

		branch = strings.TrimSpace(string(output))
		viper.Set(config.Branch, branch)
	}

	return branch, nil
}

func defaultProjectLookup(ctx context.Context, _ string) string {
	return config.Viper(ctx).GetString(config.GitProject)
}

func defaultBranchLookup(ctx context.Context, _ string) string {
	return config.Viper(ctx).GetString(config.DefaultBranch)
}
