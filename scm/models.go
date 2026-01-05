// Package scm provides source control management abstractions and provider interfaces.
package scm

// Repository represents a source code repository.
type Repository struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Public        bool     `json:"public"`
	Project       string   `json:"project"`
	DefaultBranch string   `json:"default_branch"`
	Labels        []string `json:"labels,omitempty"`
}

// PullRequest represents a pull request in a repository.
type PullRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Branch      string   `json:"branch"`
	Repo        string   `json:"repo"`
	Reviewers   []string `json:"reviewers"`
	Mergeable   bool     `json:"mergeable"`

	ID      int `json:"id"`
	Number  int `json:"number"`
	Version int `json:"version,omitempty"`
}
