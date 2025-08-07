package scm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var providerFactories = make(map[string]ProviderFactory)

type ProviderFactory func(project string) Provider

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
func Get(name, project string) Provider {
	if factory, exists := providerFactories[name]; exists {
		return factory(project)
	}

	panic(fmt.Sprintf("SCM provider %s not registered", name))
}

// Register a new SCM provider factory by name.
func Register(name string, factory ProviderFactory) {
	if _, exists := providerFactories[name]; !exists {
		providerFactories[name] = factory
	}
}

// Do executes the HTTP request and returns an error if the response status code is not successful.
func Do(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	return parseError(resp)
}

// DoResp executes the HTTP request and unmarshals the response into the provided type.
func DoResp[T any](client *http.Client, req *http.Request) (*T, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if err := parseError(resp); err != nil {
		return nil, err
	}

	var result T

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func parseError(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	output, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error %d: failed to read response body: %w", resp.StatusCode, err)
	}

	return fmt.Errorf("error %d: %s", resp.StatusCode, output)
}
