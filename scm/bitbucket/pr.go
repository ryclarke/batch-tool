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

// GetPullRequest retrieves a pull request by repository name and source branch.
func (b *Bitbucket) GetPullRequest(repo, branch string) (*scm.PullRequest, error) {
	resp, err := b.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	// Return the first PR in the results (this will be the most recent)
	return parsePR(resp), nil
}

// OpenPullRequest opens a new pull request in the specified repository.
func (b *Bitbucket) OpenPullRequest(repo, branch, title, description string, reviewers []string) (*scm.PullRequest, error) {
	// default PR title is branch name
	if title == "" {
		title = branch
	}

	payload := genPR(repo, title, description, reviewers)

	request, err := http.NewRequest(http.MethodPost, b.url(repo, nil, "pull-requests"), strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	pr, err := do[prResp](b, request)
	if err != nil {
		return nil, fmt.Errorf("failed to open pull request: %w", err)
	}

	// only ID is returned in response, so we need to set the rest of the fields for the return value
	pr.Title = title
	pr.Description = description
	pr.FromRef = prRef{
		ID: fmt.Sprintf("refs/heads/%s", branch),
		Repository: prRefRepo{
			Slug: repo, Project: prRefRepoProj{Key: b.project},
		},
	}
	pr.SetReviewers(reviewers)

	return parsePR(pr), nil
}

// UpdatePullRequest updates an existing pull request.
func (b *Bitbucket) UpdatePullRequest(repo, branch, title, description string, reviewers []string, appendReviewers bool) (*scm.PullRequest, error) {
	if title == "" && description == "" && len(reviewers) == 0 {
		return nil, fmt.Errorf("no updates provided")
	}

	pr, err := b.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	if title != "" {
		pr.Title = title
	}

	if description != "" {
		pr.Description = description
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

	request, err := http.NewRequest(http.MethodPut, b.url(repo, nil, "pull-requests", strconv.Itoa(int(pr.ID))), strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	newPR, err := do[prResp](b, request)
	if err != nil {
		return nil, fmt.Errorf("failed to update pull request: %w", err)
	}

	return parsePR(newPR), nil
}

// MergePullRequest merges an existing pull request.
func (b *Bitbucket) MergePullRequest(repo, branch string, _ bool) (*scm.PullRequest, error) {
	pr, err := b.GetPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}
	queryParams.Set("version", strconv.Itoa(pr.Version))
	req, err := http.NewRequest(http.MethodPost, b.url(repo, queryParams, "pull-requests", strconv.Itoa(pr.ID), "merge"), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if _, err := do[any](b, req); err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}

	return pr, nil
}

func (b *Bitbucket) getPullRequest(repo, branch string) (*prResp, error) {
	queryParams := url.Values{}
	queryParams.Set("direction", "outgoing")
	queryParams.Set("at", "refs/heads/"+branch)
	resp, err := get[prListResp](b, b.url(repo, queryParams, "pull-requests"))
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests for %s/%s: %w", repo, branch, err)
	}

	if len(resp.Values) == 0 {
		return nil, fmt.Errorf("no pull requests found for %s/%s", repo, branch)
	}

	return resp.Values[0], nil
}

type prListResp struct {
	Values []*prResp `json:"values"`
}

type prResp struct {
	ID      float64 `json:"id,omitempty"`
	Version float64 `json:"version,omitempty"`

	Title       string  `json:"title"`
	Description string  `json:"description"`
	Reviewers   []prRev `json:"reviewers"`
	FromRef     prRef   `json:"fromRef"`
	ToRef       prRef   `json:"toRef"`
}

type prRev struct {
	User prRevUser `json:"user"`
}

type prRevUser struct {
	Name string `json:"name"`
}

type prRef struct {
	ID         string    `json:"id"`
	Repository prRefRepo `json:"repository"`
}

type prRefRepo struct {
	Slug    string        `json:"slug"`
	Project prRefRepoProj `json:"project"`
}

type prRefRepoProj struct {
	Key string `json:"key"`
}

func (pr *prResp) GetReviewers() []string {
	output := make([]string, len(pr.Reviewers))

	for i, rev := range pr.Reviewers {
		output[i] = rev.User.Name
	}

	return output
}

func (pr *prResp) AddReviewers(reviewers []string) {
	for _, rev := range reviewers {
		pr.Reviewers = append(pr.Reviewers, prRev{
			User: prRevUser{Name: rev},
		})
	}
}

func (pr *prResp) SetReviewers(reviewers []string) {
	pr.Reviewers = make([]prRev, 0, len(reviewers))
	pr.AddReviewers(reviewers)
}

// generate a PR payload for the Bitbucket API
func genPR(name, title, description string, reviewers []string) string {
	project := viper.GetString(config.GitProject)

	pr := &prResp{
		Title:       title,
		Description: description,
		FromRef: prRef{
			ID: fmt.Sprintf("refs/heads/%s", viper.GetString(config.Branch)),
			Repository: prRefRepo{
				Slug:    name,
				Project: prRefRepoProj{Key: project},
			},
		},
		ToRef: prRef{
			ID: fmt.Sprintf("refs/heads/%s", viper.GetString(config.SourceBranch)),
			Repository: prRefRepo{
				Slug:    name,
				Project: prRefRepoProj{Key: project},
			},
		},
	}

	// generate list of reviewers
	for _, rev := range reviewers {
		pr.Reviewers = append(pr.Reviewers, prRev{
			User: prRevUser{Name: rev},
		})
	}

	output, err := json.Marshal(pr)
	if err != nil {
		return ""
	}

	return string(output)
}

func parsePR(resp *prResp) *scm.PullRequest {
	pr := &scm.PullRequest{
		ID:          int(resp.ID),
		Number:      int(resp.ID),
		Title:       resp.Title,
		Description: resp.Description,
		Reviewers:   resp.GetReviewers(),
	}

	return pr
}
