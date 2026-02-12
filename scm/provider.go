package scm

import (
	"context"
	"fmt"
)

var providerFactories = make(map[string]ProviderFactory)

// ProviderFactory is a function that creates a new Provider instance.
type ProviderFactory func(ctx context.Context, project string) Provider

// Provider defines the interface for SCM providers.
type Provider interface {
	// CheckCapabilities validates that the provided PR options are supported by the provider.
	CheckCapabilities(opts *PROptions) error

	// ListRepositories lists all repositories in the specified project.
	ListRepositories() ([]*Repository, error)

	// GetPullRequest retrieves a pull request by repository name and source branch.
	GetPullRequest(repo, branch string) (*PullRequest, error)
	// OpenPullRequest opens a new pull request in the specified repository.
	OpenPullRequest(repo, branch string, opts *PROptions) (*PullRequest, error)
	// UpdatePullRequest updates an existing pull request.
	UpdatePullRequest(repo, branch string, opts *PROptions) (*PullRequest, error)
	// MergePullRequest merges an existing pull request.
	MergePullRequest(repo, branch string, opts *PRMergeOptions) (*PullRequest, error)
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
