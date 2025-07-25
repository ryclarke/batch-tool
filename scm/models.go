package scm

type Repository struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Public        bool     `json:"public"`
	Project       string   `json:"project"`
	DefaultBranch string   `json:"default_branch"`
	Labels        []string `json:"labels,omitempty"`
}

type PullRequest map[string]any

// ID of the PR
func (pr PullRequest) ID() int {
	if id, ok := pr["id"]; ok {
		return int(id.(float64))
	}

	return 0
}

// Version of the PR
func (pr PullRequest) Version() int {
	if version, ok := pr["version"]; ok {
		return int(version.(float64))
	}

	return 0
}

// AddReviewers appends the given list of reviewers to the PR
func (pr PullRequest) AddReviewers(reviewers []string) {
	for _, rev := range reviewers {
		pr["reviewers"] = append(pr["reviewers"].([]any), map[string]any{
			"user": map[string]any{"name": rev},
		})
	}
}

// SetReviewers sets the PR reviewers to the given list
func (pr PullRequest) SetReviewers(reviewers []string) {
	pr["reviewers"] = make([]any, 0, len(reviewers))
	pr.AddReviewers(reviewers)
}

// GetReviewers returns the list of reviewers for the PR
func (pr PullRequest) GetReviewers() []string {
	revs := pr["reviewers"].([]any)
	output := make([]string, len(revs))

	for i, rev := range revs {
		output[i] = rev.(map[string]any)["user"].(map[string]any)["name"].(string)
	}

	return output
}
