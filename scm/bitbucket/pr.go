package bitbucket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

type prListResp struct {
	Values []scm.PullRequest `json:"values"`
}

type prIDResp struct {
	ID int `json:"id"`
}

// GetPullRequest retrieves a pull request by repository name and source branch.
func (b *Bitbucket) GetPullRequest(repo, branch string) (scm.PullRequest, error) {
	prs, err := b.getPullRequests(repo, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests for %s/%s: %w", repo, branch, err)
	}

	if len(prs) == 0 {
		return nil, fmt.Errorf("no pull requests found for %s/%s", repo, branch)
	}

	// Return the first PR in the results (this will be the most recent)
	return prs[0], nil
}

// OpenPullRequest opens a new pull request in the specified repository.
func (b *Bitbucket) OpenPullRequest(repo, branch, title, description string, reviewers []string) (scm.PullRequest, error) {
	// default PR title is branch name
	if title == "" {
		title = branch
	}

	payload := genPR(repo, title, description, reviewers)

	request, err := http.NewRequest(http.MethodPost, b.url(repo, nil, "pull-requests"), strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	pr, err := do[prIDResp](b, request)
	if err != nil {
		return nil, fmt.Errorf("failed to open pull request: %w", err)
	}

	return scm.PullRequest{
		"id":          pr.ID,
		"title":       title,
		"description": description,
		"repo":        fmt.Sprintf("%s/%s", b.project, repo),
		"branch":      branch,
		"reviewers":   reviewers,
	}, nil
}

// UpdatePullRequest updates an existing pull request.
func (b *Bitbucket) UpdatePullRequest(repo, branch, title, description string, reviewers []string, appendReviewers bool) (scm.PullRequest, error) {
	if title == "" && description == "" && len(reviewers) == 0 {
		return nil, fmt.Errorf("no updates provided")
	}

	pr, err := b.GetPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	delete(pr, "participants")
	delete(pr, "author")

	if title != "" {
		pr["title"] = title
	}

	if description != "" {
		pr["description"] = description
	}

	if len(reviewers) == 0 {
		if appendReviewers {
			pr.AddReviewers(reviewers)
		} else if len(viper.GetStringSlice(config.Reviewers)) > 0 {
			pr.SetReviewers(reviewers)
		}
	}

	payload, err := json.Marshal(pr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pull request payload: %w", err)
	}

	request, err := http.NewRequest(http.MethodPut, b.url(repo, nil, "pull-requests", strconv.Itoa(pr.ID())), strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	newPR, err := do[scm.PullRequest](b, request)
	if err != nil {
		return nil, fmt.Errorf("failed to update pull request: %w", err)
	}

	return *newPR, nil
}

// MergePullRequest merges an existing pull request.
func (b *Bitbucket) MergePullRequest(repo, branch string) (scm.PullRequest, error) {
	pr, err := b.GetPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}
	queryParams.Set("version", strconv.Itoa(pr.Version()))
	req, err := http.NewRequest(http.MethodPost, b.url(repo, queryParams, "pull-requests", strconv.Itoa(pr.ID()), "merge"), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if _, err := do[any](b, req); err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}

	return pr, nil
}

func (b *Bitbucket) getPullRequests(repo, branch string) ([]scm.PullRequest, error) {
	queryParams := url.Values{}
	queryParams.Set("direction", "outgoing")
	queryParams.Set("at", "refs/heads/"+branch)
	resp, err := get[prListResp](b, b.url(repo, queryParams, "pull-requests"))
	if err != nil {
		return nil, err
	}

	return resp.Values, nil
}

// generate a PR payload for the Bitbucket API
func genPR(name, title, description string, reviewers []string) string {
	project := viper.GetString(config.GitProject)

	// generate list of reviewers
	revs := make([]string, 0, len(reviewers))
	for _, rev := range reviewers {
		if rev != "" {
			revs = append(revs, fmt.Sprintf(config.PrReviewerTmpl, rev))
		}
	}

	return fmt.Sprintf(config.PrTmpl, title, description,
		viper.GetString(config.Branch), fmt.Sprintf(config.PrRepoTmpl, name, project),
		viper.GetString(config.SourceBranch), fmt.Sprintf(config.PrRepoTmpl, name, project),
		strings.Join(revs, ","),
	)
}
