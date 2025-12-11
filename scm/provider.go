package scm

import (
	"context"
	"fmt"
)

var providerFactories = make(map[string]ProviderFactory)

type ProviderFactory func(ctx context.Context, project string) Provider

// Provider defines the interface for SCM providers.
type Provider interface {
	// ListRepositories lists all repositories in the specified project.
	ListRepositories() ([]*Repository, error)

	// GetPullRequest retrieves a pull request by repository name and source branch.
	GetPullRequest(repo, branch string) (*PullRequest, error)
	// OpenPullRequest opens a new pull request in the specified repository.
	OpenPullRequest(repo, branch, title, description string, reviewers []string) (*PullRequest, error)
	// UpdatePullRequest updates an existing pull request.
	UpdatePullRequest(repo, branch, title, description string, reviewers []string, appendReviewers bool) (*PullRequest, error)
	// MergePullRequest merges an existing pull request.
	MergePullRequest(repo, branch string) (*PullRequest, error)
}

// Get retrieves a registered SCM provider by name.
// If the provider is not registered, it panics.
func Get(ctx context.Context, name, project string) Provider {
	if factory, exists := providerFactories[name]; exists {
		return factory(ctx, project)
	}

	panic(fmt.Sprintf("SCM provider %s not registered", name))
}

// Register a new SCM provider factory by name.
func Register(name string, factory ProviderFactory) {
	if _, exists := providerFactories[name]; !exists {
		providerFactories[name] = factory
	}
}
