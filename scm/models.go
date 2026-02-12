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
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Branch        string   `json:"branch"`
	Repo          string   `json:"repo"`
	Reviewers     []string `json:"reviewers"`
	TeamReviewers []string `json:"team_reviewers,omitempty"`

	ID        int  `json:"id"`
	Number    int  `json:"number"`
	Version   int  `json:"version,omitempty"`
	Draft     bool `json:"draft,omitempty"`
	Mergeable bool `json:"mergeable"`
}

// PROptions holds options for creating or updating pull requests.
type PROptions struct {
	Title          string
	Description    string
	Reviewers      []string
	TeamReviewers  []string
	ResetReviewers bool
	BaseBranch     string
	Draft          *bool

	Merge PRMergeOptions
}

// PRMergeOptions holds options for merging pull requests.
type PRMergeOptions struct {
	Method         string
	CheckMergeable bool
}
