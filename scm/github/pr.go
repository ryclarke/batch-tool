// Package github provides GitHub SCM integration for batch-tool.
package github

import (
	"fmt"

	"github.com/google/go-github/v74/github"

	"github.com/ryclarke/batch-tool/scm"
)

// GetPullRequest retrieves a pull request by repository name and source branch.
func (g *Github) GetPullRequest(repo, branch string) (*scm.PullRequest, error) {
	resp, err := g.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	return parsePR(resp), nil
}

// OpenPullRequest opens a new pull request in the specified repository.
func (g *Github) OpenPullRequest(repo, branch string, opts *scm.PROptions) (*scm.PullRequest, error) {
	if opts == nil {
		opts = &scm.PROptions{} // default options
	}

	// reads are less restrictive than a failed write, so check for existing PR first
	if _, err := g.getPullRequest(repo, branch); err == nil {
		return nil, fmt.Errorf("a pull request already exists for branch %s in repository %s", branch, repo)
	}

	// if title is not specified, use the branch name
	if opts.Title == "" {
		opts.Title = branch
	}

	req := &github.NewPullRequest{
		Head:  github.Ptr(branch),
		Base:  github.Ptr(opts.BaseBranch),
		Title: github.Ptr(opts.Title),
		Body:  github.Ptr(opts.Description),
	}

	// if Draft status is specified, apply it to the PR
	if opts.Draft != nil {
		req.Draft = github.Ptr(*opts.Draft)
	}

	resp, err := g.openPullRequest(repo, req)
	if err != nil {
		return nil, err
	}

	opts.ResetReviewers = false // suppress ResetReviewers when opening a new PR
	if resp, err = g.applyReviewers(repo, resp, opts); err != nil {
		return nil, err
	}

	return parsePR(resp), nil
}

// UpdatePullRequest updates an existing pull request.
func (g *Github) UpdatePullRequest(repo, branch string, opts *scm.PROptions) (*scm.PullRequest, error) {
	pr, err := g.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	req, changes := g.processChanges(opts)
	if changes {
		if pr, err = g.editPullRequest(repo, pr.GetNumber(), req); err != nil {
			return nil, err
		}
	}

	if pr, err = g.applyReviewers(repo, pr, opts); err != nil {
		return nil, err
	}

	return parsePR(pr), nil
}

// MergePullRequest merges an existing pull request
func (g *Github) MergePullRequest(repo, branch string, force bool) (*scm.PullRequest, error) {
	pr, err := g.getPullRequest(repo, branch)
	if err != nil {
		return nil, err
	}

	if !force && !pr.GetMergeable() {
		return nil, fmt.Errorf("pull request %s [%d] for %s is not mergeable: %s", branch, pr.GetNumber(), repo, pr.GetMergeableState())
	}

	if err = g.mergePullRequest(repo, pr.GetNumber()); err != nil {
		return nil, err
	}

	return parsePR(pr), nil
}

func (g *Github) getPullRequest(repo, branch string) (*github.PullRequest, error) {
	// acquire read lock (and release it when done)
	defer g.readLock()()

	opts := &github.PullRequestListOptions{
		State: "open",
		Head:  fmt.Sprintf("%s:%s", g.project, branch),
	}

	resp, _, err := g.client.PullRequests.List(g.ctx, g.project, repo, opts)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, true); rateErr != nil {
			return nil, fmt.Errorf("failed to get pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to get pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.List(g.ctx, g.project, repo, opts); err != nil {
			return nil, fmt.Errorf("failed to get pull request after retry: %w", err)
		}
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("no open pull request found for branch %s in repository %s", branch, repo)
	}

	return resp[0], nil
}

func (g *Github) getPullRequestByNumber(repo string, prNumber int) (*github.PullRequest, error) {
	// acquire read lock (and release it when done)
	defer g.readLock()()

	resp, _, err := g.client.PullRequests.Get(g.ctx, g.project, repo, prNumber)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, true); rateErr != nil {
			return nil, fmt.Errorf("failed to get pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to get pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.Get(g.ctx, g.project, repo, prNumber); err != nil {
			return nil, fmt.Errorf("failed to get pull request after retry: %w", err)
		}
	}

	return resp, nil
}

func (g *Github) openPullRequest(repo string, req *github.NewPullRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	resp, _, err := g.client.PullRequests.Create(g.ctx, g.project, repo, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to open pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to open pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.Create(g.ctx, g.project, repo, req); err != nil {
			return nil, fmt.Errorf("failed to open pull request after retry: %w", err)
		}
	}

	return resp, nil
}

func (g *Github) editPullRequest(repo string, prNumber int, req *github.PullRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	pr, _, err := g.client.PullRequests.Edit(g.ctx, g.project, repo, prNumber, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to update pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to update pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if pr, _, err = g.client.PullRequests.Edit(g.ctx, g.project, repo, prNumber, req); err != nil {
			return nil, fmt.Errorf("failed to update pull request after retry: %w", err)
		}
	}

	return pr, nil
}

func (g *Github) mergePullRequest(repo string, prNumber int) error {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	_, _, err := g.client.PullRequests.Merge(g.ctx, g.project, repo, prNumber, "", nil)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return fmt.Errorf("failed to merge pull request: %w: %w", rateErr, err)
		} else if !retry {
			return fmt.Errorf("failed to merge pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if _, _, err = g.client.PullRequests.Merge(g.ctx, g.project, repo, prNumber, "", nil); err != nil {
			return fmt.Errorf("failed to merge pull request after retry: %w", err)
		}
	}

	return nil
}

func (g *Github) processChanges(opts *scm.PROptions) (req *github.PullRequest, changed bool) {
	req = &github.PullRequest{}

	if opts.Title != "" {
		req.Title = github.Ptr(opts.Title)
		changed = true
	}

	if opts.Description != "" {
		req.Body = github.Ptr(opts.Description)
		changed = true
	}

	if opts.Draft != nil {
		req.Draft = github.Ptr(*opts.Draft)
		changed = true
	}

	return req, changed
}

func parsePR(resp *github.PullRequest) *scm.PullRequest {
	pr := &scm.PullRequest{
		ID:          int(resp.GetID()),
		Number:      resp.GetNumber(),
		Title:       resp.GetTitle(),
		Description: resp.GetBody(),
		Reviewers:   make([]string, 0, len(resp.RequestedReviewers)),
		Draft:       resp.GetDraft(),
	}

	for _, reviewer := range resp.RequestedReviewers {
		pr.Reviewers = append(pr.Reviewers, reviewer.GetLogin())
	}

	return pr
}
