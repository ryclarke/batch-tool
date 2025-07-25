package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v74/github"
	"github.com/ryclarke/batch-tool/scm"
)

// GetPullRequest retrieves a pull request by repository name and source branch.
func (g *Github) GetPullRequest(repo, branch string) (scm.PullRequest, error) {
	resp, err := g.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	pr := scm.PullRequest{
		"id":          resp.GetID(),
		"number":      resp.GetNumber(),
		"title":       resp.GetTitle(),
		"description": resp.GetBody(),
		"reviewers":   make([]string, 0),
	}

	for _, reviewer := range resp.RequestedReviewers {
		pr["reviewers"] = append(pr["reviewers"].([]string), reviewer.GetLogin())
	}

	return pr, nil
}

// OpenPullRequest opens a new pull request in the specified repository.
func (g *Github) OpenPullRequest(repo, branch, title, description string, reviewers []string) (scm.PullRequest, error) {
	resp, _, err := g.client.PullRequests.Create(context.TODO(), g.project, repo, &github.NewPullRequest{
		Title: &title,
		Body:  &description,
		Head:  &branch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open pull request: %w", err)
	}

	resp, _, err = g.client.PullRequests.RequestReviewers(context.TODO(), g.project, repo, resp.GetNumber(), github.ReviewersRequest{
		Reviewers: reviewers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to request reviewers: %w", err)
	}

	return parsePR(resp), nil
}

// UpdatePullRequest updates an existing pull request.
func (g *Github) UpdatePullRequest(repo, branch, title, description string, reviewers []string, appendReviewers bool) (scm.PullRequest, error) {
	pr, err := g.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	if title != "" || description != "" {
		pr, _, err = g.client.PullRequests.Edit(context.TODO(), g.project, repo, pr.GetNumber(), &github.PullRequest{
			Title: &title,
			Body:  &description,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update pull request: %w", err)
		}
	}

	if len(reviewers) > 0 {
		pr, _, err = g.client.PullRequests.RequestReviewers(context.TODO(), g.project, repo, pr.GetNumber(), github.ReviewersRequest{
			Reviewers: reviewers,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to request reviewers: %w", err)
		}
	}

	return parsePR(pr), nil
}

func (g *Github) MergePullRequest(repo, branch string) (scm.PullRequest, error) {
	pr, err := g.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	if !pr.GetMergeable() {
		return nil, fmt.Errorf("pull request %s [%d] for %s is not mergeable: %s", branch, pr.GetNumber(), repo, pr.GetMergeableState())
	}

	_, _, err = g.client.PullRequests.Merge(context.TODO(), g.project, repo, pr.GetNumber(), "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}

	// Implementation goes here
	return parsePR(pr), nil
}

func (g *Github) getPullRequest(repo, branch string) (*github.PullRequest, error) {
	resp, _, err := g.client.PullRequests.List(context.TODO(), g.project, repo, &github.PullRequestListOptions{
		State: "open",
		Head:  branch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests: %w", err)
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("no open pull request found for branch %s in repository %s", branch, repo)
	}

	return resp[0], nil
}

func parsePR(resp *github.PullRequest) scm.PullRequest {
	pr := scm.PullRequest{
		"id":          resp.GetID(),
		"number":      resp.GetNumber(),
		"title":       resp.GetTitle(),
		"description": resp.GetBody(),
		"reviewers":   make([]string, 0),
	}

	for _, reviewer := range resp.RequestedReviewers {
		pr["reviewers"] = append(pr["reviewers"].([]string), reviewer.GetLogin())
	}

	return pr
}
