package fake

import (
	"context"
	"fmt"
	"maps"
	"sort"

	"github.com/ryclarke/batch-tool/scm"
)

var _ scm.Provider = new(Fake)

func init() {
	// Register the fake provider factory
	scm.Register("fake", New)
}

// Fake implements a mock SCM provider for testing purposes
type Fake struct {
	Project      string
	Repositories []*scm.Repository
	PullRequests map[string]*scm.PullRequest // key: "repo:branch"
	Errors       map[string]error            // configurable errors for testing
	Capabilities *scm.Capabilities           // configurable capabilities for testing
}

// New creates a new fake SCM provider with the specified project
func New(_ context.Context, project string) scm.Provider {
	return &Fake{
		Project:      project,
		Repositories: make([]*scm.Repository, 0),
		PullRequests: make(map[string]*scm.PullRequest),
		Errors:       make(map[string]error),
		Capabilities: &scm.Capabilities{
			TeamReviewers:  true,
			ResetReviewers: true,
			Draft:          true,
			MergeMethods:   []string{"merge", "squash", "rebase"},
			CheckMergeable: true,
		},
	}
}

// NewFake creates a new fake SCM provider with the specified project and optional seed data.
// Optionally pass a Capabilities struct to override the default (which supports all features).
func NewFake(project string, repos []*scm.Repository, caps ...*scm.Capabilities) *Fake {
	f := New(context.Background(), project).(*Fake)
	f.Repositories = make([]*scm.Repository, len(repos))

	// Deep copy repositories to avoid mutations affecting tests
	for i, repo := range repos {
		f.Repositories[i] = &scm.Repository{
			Name:          repo.Name,
			Description:   repo.Description,
			Public:        repo.Public,
			Project:       repo.Project,
			DefaultBranch: repo.DefaultBranch,
			Labels:        append([]string(nil), repo.Labels...), // copy slice
		}
	}

	// Override capabilities if provided
	if len(caps) > 0 {
		f.Capabilities = caps[0]
	}

	return f
}

// SeedErrors configures the provider to return specific errors for testing
func (f *Fake) SeedErrors(errors map[string]error) {
	// Deep copy errors to avoid mutations affecting tests
	maps.Copy(f.Errors, errors)
}

// CheckCapabilities validates that the provided PR options are supported by the fake provider.
func (f *Fake) CheckCapabilities(opts *scm.PROptions) error {
	return scm.ValidatePROptions(f.Capabilities, opts)
}

// ListRepositories returns the configured repositories
func (f *Fake) ListRepositories() ([]*scm.Repository, error) {
	if err := f.Errors["ListRepositories"]; err != nil {
		return nil, err
	}

	// Return a copy to prevent mutations
	result := make([]*scm.Repository, len(f.Repositories))
	for i, repo := range f.Repositories {
		result[i] = &scm.Repository{
			Name:          repo.Name,
			Description:   repo.Description,
			Public:        repo.Public,
			Project:       repo.Project,
			DefaultBranch: repo.DefaultBranch,
			Labels:        append([]string(nil), repo.Labels...),
		}
	}

	return result, nil
}

// GetPullRequest retrieves a pull request by repository name and source branch
func (f *Fake) GetPullRequest(repo, branch string) (*scm.PullRequest, error) {
	if err := f.Errors["GetPullRequest"]; err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s:%s", repo, branch)
	if pr, exists := f.PullRequests[key]; exists {
		// Return a copy to prevent mutations
		result := &scm.PullRequest{
			Title:         pr.Title,
			Description:   pr.Description,
			Branch:        pr.Branch,
			Repo:          pr.Repo,
			Reviewers:     make([]string, 0, len(pr.Reviewers)),
			TeamReviewers: make([]string, 0, len(pr.TeamReviewers)),
			Mergeable:     pr.Mergeable,
			ID:            pr.ID,
			Number:        pr.Number,
			Version:       pr.Version,
			Draft:         pr.Draft,
		}

		result.Reviewers = append(result.Reviewers, pr.Reviewers...)
		result.TeamReviewers = append(result.TeamReviewers, pr.TeamReviewers...)

		return result, nil
	}

	return nil, fmt.Errorf("pull request not found for %s:%s", repo, branch)
}

// OpenPullRequest creates a new pull request
func (f *Fake) OpenPullRequest(repo, branch string, opts *scm.PROptions) (*scm.PullRequest, error) {
	if opts == nil {
		opts = &scm.PROptions{} // default options
	}

	if err := f.Errors["OpenPullRequest"]; err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s:%s", repo, branch)

	// Check if PR already exists
	if _, exists := f.PullRequests[key]; exists {
		return nil, fmt.Errorf("pull request already exists for %s:%s", repo, branch)
	}

	// Create new PR
	prID := len(f.PullRequests) + 1
	pr := &scm.PullRequest{
		ID:            prID,
		Version:       1,
		Title:         opts.Title,
		Description:   opts.Description,
		Branch:        branch,
		Repo:          repo,
		Reviewers:     opts.Reviewers,
		TeamReviewers: opts.TeamReviewers,
		Mergeable:     true, // Default to mergeable
	}

	f.PullRequests[key] = pr

	// Return a copy
	return copyPR(pr), nil
}

// UpdatePullRequest updates an existing pull request
func (f *Fake) UpdatePullRequest(repo, branch string, opts *scm.PROptions) (*scm.PullRequest, error) {
	if opts == nil {
		opts = &scm.PROptions{} // default options
	}

	if err := f.Errors["UpdatePullRequest"]; err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s:%s", repo, branch)

	pr, exists := f.PullRequests[key]
	if !exists {
		return nil, fmt.Errorf("pull request not found for %s:%s", repo, branch)
	}

	// Update fields
	pr.Title = opts.Title
	pr.Description = opts.Description

	// Increment version
	pr.Version++

	// Update reviewers
	if opts.ResetReviewers {
		pr.Reviewers = opts.Reviewers
		pr.TeamReviewers = opts.TeamReviewers
	} else {
		allReviewers := append(pr.Reviewers, opts.Reviewers...)

		// Remove duplicates
		reviewerSet := make(map[string]bool)
		uniqueReviewers := make([]string, 0)
		for _, reviewer := range allReviewers {
			if !reviewerSet[reviewer] {
				reviewerSet[reviewer] = true
				uniqueReviewers = append(uniqueReviewers, reviewer)
			}
		}
		pr.Reviewers = uniqueReviewers

		// Do the same for team reviewers
		allTeamReviewers := append(pr.TeamReviewers, opts.TeamReviewers...)
		teamReviewerSet := make(map[string]bool)
		uniqueTeamReviewers := make([]string, 0)
		for _, teamReviewer := range allTeamReviewers {
			if !teamReviewerSet[teamReviewer] {
				teamReviewerSet[teamReviewer] = true
				uniqueTeamReviewers = append(uniqueTeamReviewers, teamReviewer)
			}
		}
		pr.TeamReviewers = uniqueTeamReviewers
	}

	// Return a copy
	return copyPR(pr), nil
}

// MergePullRequest merges an existing pull request
func (f *Fake) MergePullRequest(repo, branch string, opts *scm.PRMergeOptions) (*scm.PullRequest, error) {
	if err := f.Errors["MergePullRequest"]; err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &scm.PRMergeOptions{} // default options
	}

	key := fmt.Sprintf("%s:%s", repo, branch)

	pr, exists := f.PullRequests[key]
	if !exists {
		return nil, fmt.Errorf("pull request not found for %s:%s", repo, branch)
	}

	// Check mergeability if check flag is enabled
	if opts.CheckMergeable && !pr.Mergeable {
		return nil, fmt.Errorf("pull request for %s:%s is not mergeable (conflicts, required checks failing, etc)", repo, branch)
	}

	delete(f.PullRequests, key)

	// Return a copy
	return copyPR(pr), nil
}

// Test helper methods for configuring the fake provider

// AddRepository adds a repository to the fake provider
func (f *Fake) AddRepository(repo *scm.Repository) {
	f.Repositories = append(f.Repositories, &scm.Repository{
		Name:          repo.Name,
		Description:   repo.Description,
		Public:        repo.Public,
		Project:       repo.Project,
		DefaultBranch: repo.DefaultBranch,
		Labels:        append([]string(nil), repo.Labels...),
	})
}

// AddRepositories adds multiple repositories to the fake provider
func (f *Fake) AddRepositories(repos ...*scm.Repository) {
	for _, repo := range repos {
		f.AddRepository(repo)
	}
}

// SetError configures the provider to return an error for a specific method
func (f *Fake) SetError(method string, err error) {
	f.Errors[method] = err
}

// ClearError removes a configured error for a specific method
func (f *Fake) ClearError(method string) {
	delete(f.Errors, method)
}

// ClearAllErrors removes all configured errors
func (f *Fake) ClearAllErrors() {
	f.Errors = make(map[string]error)
}

// SetPRMergeable sets the mergeable status of a pull request for testing
func (f *Fake) SetPRMergeable(repo, branch string, mergeable bool) error {
	key := fmt.Sprintf("%s:%s", repo, branch)
	pr, exists := f.PullRequests[key]
	if !exists {
		return fmt.Errorf("pull request not found for %s:%s", repo, branch)
	}
	pr.Mergeable = mergeable
	return nil
}

// GetRepositoryCount returns the number of repositories in the fake provider
func (f *Fake) GetRepositoryCount() int {
	return len(f.Repositories)
}

// GetPullRequestCount returns the number of pull requests in the fake provider
func (f *Fake) GetPullRequestCount() int {
	return len(f.PullRequests)
}

// HasPullRequest checks if a pull request exists for the given repo and branch
func (f *Fake) HasPullRequest(repo, branch string) bool {
	key := fmt.Sprintf("%s:%s", repo, branch)
	_, exists := f.PullRequests[key]
	return exists
}

// Clear removes all repositories and pull requests
func (f *Fake) Clear() {
	f.Repositories = make([]*scm.Repository, 0)
	f.PullRequests = make(map[string]*scm.PullRequest)
	f.Errors = make(map[string]error)
}

// CreateTestRepositories creates a set of test repositories with various labels
func CreateTestRepositories(project string) []*scm.Repository {
	return []*scm.Repository{
		{
			Name:          "repo-1",
			Description:   "Test repository 1",
			Public:        true,
			Project:       project,
			DefaultBranch: "main",
			Labels:        []string{"backend", "go", "active"},
		},
		{
			Name:          "repo-2",
			Description:   "Test repository 2",
			Public:        false,
			Project:       project,
			DefaultBranch: "master",
			Labels:        []string{"frontend", "javascript", "active"},
		},
		{
			Name:          "repo-3",
			Description:   "Test repository 3",
			Public:        true,
			Project:       project,
			DefaultBranch: "main",
			Labels:        []string{"deprecated", "legacy"},
		},
		{
			Name:          "repo-4",
			Description:   "Test repository 4",
			Public:        true,
			Project:       project,
			DefaultBranch: "develop",
			Labels:        []string{"poc", "experimental"},
		},
		{
			Name:          "repo-5",
			Description:   "Test repository 5",
			Public:        false,
			Project:       project,
			DefaultBranch: "main",
			Labels:        []string{"backend", "python", "active", "microservice"},
		},
	}
}

// GetRepositoryByName returns a repository by name, or nil if not found
func (f *Fake) GetRepositoryByName(name string) *scm.Repository {
	for _, repo := range f.Repositories {
		if repo.Name == name {
			// Return a copy
			return &scm.Repository{
				Name:          repo.Name,
				Description:   repo.Description,
				Public:        repo.Public,
				Project:       repo.Project,
				DefaultBranch: repo.DefaultBranch,
				Labels:        append([]string(nil), repo.Labels...),
			}
		}
	}
	return nil
}

// GetRepositoriesByLabel returns repositories that have the specified label
func (f *Fake) GetRepositoriesByLabel(label string) []*scm.Repository {
	var result []*scm.Repository
	for _, repo := range f.Repositories {
		for _, repoLabel := range repo.Labels {
			if repoLabel == label {
				result = append(result, &scm.Repository{
					Name:          repo.Name,
					Description:   repo.Description,
					Public:        repo.Public,
					Project:       repo.Project,
					DefaultBranch: repo.DefaultBranch,
					Labels:        append([]string(nil), repo.Labels...),
				})
				break
			}
		}
	}
	return result
}

// GetAllLabels returns all unique labels across all repositories
func (f *Fake) GetAllLabels() []string {
	labelSet := make(map[string]bool)
	for _, repo := range f.Repositories {
		for _, label := range repo.Labels {
			labelSet[label] = true
		}
	}

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}

	sort.Strings(labels)
	return labels
}

func copyPR(pr *scm.PullRequest) *scm.PullRequest {
	// Return a copy to prevent mutations
	result := &scm.PullRequest{
		Title:         pr.Title,
		Description:   pr.Description,
		Branch:        pr.Branch,
		Repo:          pr.Repo,
		Reviewers:     make([]string, 0, len(pr.Reviewers)),
		TeamReviewers: make([]string, 0, len(pr.TeamReviewers)),
		ID:            pr.ID,
		Number:        pr.Number,
		Version:       pr.Version,
		Draft:         pr.Draft,
		Mergeable:     pr.Mergeable,
	}

	result.Reviewers = append(result.Reviewers, pr.Reviewers...)
	result.TeamReviewers = append(result.TeamReviewers, pr.TeamReviewers...)

	return result
}
