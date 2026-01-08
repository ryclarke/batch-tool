// Package github provides GitHub SCM integration for batch-tool.
package github

import (
	"context"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/go-github/v74/github"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/scm"
)

// GetPullRequest retrieves a pull request by repository name and source branch.
func (g *Github) GetPullRequest(repo, branch string) (*scm.PullRequest, error) {
	resp, err := g.getPullRequest(context.TODO(), repo, branch)
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
	if _, err := g.getPullRequest(context.TODO(), repo, branch); err == nil {
		return nil, fmt.Errorf("a pull request already exists for branch %s in repository %s", branch, repo)
	}

	// check default branch for the current repo if base branch is not specified
	if opts.BaseBranch == "" {
		opts.BaseBranch = catalog.GetBranchForRepo(g.ctx, repo)
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
		Draft: github.Ptr(opts.Draft),
	}

	resp, err := g.openPullRequest(context.TODO(), repo, req)
	if err != nil {
		return nil, err
	}

	if len(opts.Reviewers) > 0 {
		if resp, err = g.requestReviewers(context.TODO(), repo, resp.GetNumber(), github.ReviewersRequest{Reviewers: opts.Reviewers}); err != nil {
			return nil, err
		}
	}

	return parsePR(resp), nil
}

// UpdatePullRequest updates an existing pull request.
func (g *Github) UpdatePullRequest(repo, branch string, opts *scm.PROptions) (*scm.PullRequest, error) {
	pr, err := g.getPullRequest(context.TODO(), repo, branch)
	if err != nil {
		return nil, err
	}

	if opts.Title != "" || opts.Description != "" {
		req := &github.PullRequest{
			Title: &opts.Title,
			Body:  &opts.Description,
		}

		if pr, err = g.editPullRequest(context.TODO(), repo, pr.GetNumber(), req); err != nil {
			return nil, err
		}
	}

	if len(opts.Reviewers) > 0 {
		// If ResetReviewers is true, replace existing reviewers
		if opts.ResetReviewers {
			if err = g.replaceReviewers(context.TODO(), repo, pr.GetNumber(), opts.Reviewers); err != nil {
				return nil, err
			}
			// Refresh PR to get updated reviewer list
			if pr, err = g.getPullRequest(context.TODO(), repo, branch); err != nil {
				return nil, err
			}
		} else {
			// GitHub's RequestReviewers API appends to existing reviewers
			if pr, err = g.requestReviewers(context.TODO(), repo, pr.GetNumber(), github.ReviewersRequest{Reviewers: opts.Reviewers}); err != nil {
				return nil, err
			}
		}
	}

	return parsePR(pr), nil
}

// MergePullRequest merges an existing pull request
func (g *Github) MergePullRequest(repo, branch string, force bool) (*scm.PullRequest, error) {
	pr, err := g.getPullRequest(context.TODO(), repo, branch)
	if err != nil {
		return nil, err
	}

	if !force && !pr.GetMergeable() {
		return nil, fmt.Errorf("pull request %s [%d] for %s is not mergeable: %s", branch, pr.GetNumber(), repo, pr.GetMergeableState())
	}

	if err = g.mergePullRequest(context.TODO(), repo, pr.GetNumber()); err != nil {
		return nil, err
	}

	return parsePR(pr), nil
}

func (g *Github) getPullRequest(ctx context.Context, repo, branch string) (*github.PullRequest, error) {
	// acquire read lock (and release it when done)
	defer g.readLock()()

	opts := &github.PullRequestListOptions{
		State: "open",
		Head:  fmt.Sprintf("%s:%s", g.project, branch),
	}

	resp, _, err := g.client.PullRequests.List(ctx, g.project, repo, opts)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, true); rateErr != nil {
			return nil, fmt.Errorf("failed to get pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to get pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.List(ctx, g.project, repo, opts); err != nil {
			return nil, fmt.Errorf("failed to get pull request after retry: %w", err)
		}
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("no open pull request found for branch %s in repository %s", branch, repo)
	}

	return resp[0], nil
}

func (g *Github) openPullRequest(ctx context.Context, repo string, req *github.NewPullRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	resp, _, err := g.client.PullRequests.Create(ctx, g.project, repo, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to open pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to open pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.Create(ctx, g.project, repo, req); err != nil {
			return nil, fmt.Errorf("failed to open pull request after retry: %w", err)
		}
	}

	return resp, nil
}

func (g *Github) editPullRequest(ctx context.Context, repo string, prNumber int, req *github.PullRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	pr, _, err := g.client.PullRequests.Edit(ctx, g.project, repo, prNumber, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to update pull request: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to update pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if pr, _, err = g.client.PullRequests.Edit(ctx, g.project, repo, prNumber, req); err != nil {
			return nil, fmt.Errorf("failed to update pull request after retry: %w", err)
		}
	}

	return pr, nil
}

func (g *Github) requestReviewers(ctx context.Context, repo string, prNumber int, req github.ReviewersRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	resp, _, err := g.client.PullRequests.RequestReviewers(ctx, g.project, repo, prNumber, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to request reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to request reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.RequestReviewers(ctx, g.project, repo, prNumber, req); err != nil {
			return nil, fmt.Errorf("failed to request reviewers after retry: %w", err)
		}
	}

	return resp, nil
}

func (g *Github) listReviewers(ctx context.Context, repo string, prNumber int) ([]string, error) {
	// acquire read lock (and release it when done)
	defer g.readLock()()

	reviewers, _, err := g.client.PullRequests.ListReviewers(ctx, g.project, repo, prNumber, nil)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, true); rateErr != nil {
			return nil, fmt.Errorf("failed to list reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to list reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if reviewers, _, err = g.client.PullRequests.ListReviewers(ctx, g.project, repo, prNumber, nil); err != nil {
			return nil, fmt.Errorf("failed to list reviewers after retry: %w", err)
		}
	}

	result := make([]string, 0, len(reviewers.Users))
	for _, user := range reviewers.Users {
		result = append(result, user.GetLogin())
	}

	return result, nil
}

func (g *Github) removeReviewers(ctx context.Context, repo string, prNumber int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	// acquire write lock (and release it when done)
	defer g.writeLock()()

	_, err := g.client.PullRequests.RemoveReviewers(ctx, g.project, repo, prNumber, github.ReviewersRequest{Reviewers: reviewers})
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, false); rateErr != nil {
			return fmt.Errorf("failed to remove reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return fmt.Errorf("failed to remove reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if _, err = g.client.PullRequests.RemoveReviewers(ctx, g.project, repo, prNumber, github.ReviewersRequest{Reviewers: reviewers}); err != nil {
			return fmt.Errorf("failed to remove reviewers after retry: %w", err)
		}
	}

	return nil
}

// replaceReviewers replaces the current reviewers with the provided list
func (g *Github) replaceReviewers(ctx context.Context, repo string, prNumber int, newReviewers []string) error {
	// Get current reviewers
	currentReviewers, err := g.listReviewers(ctx, repo, prNumber)
	if err != nil {
		return err
	}

	// Build sets for comparison
	currentSet := mapset.NewSet(currentReviewers...)
	newSet := mapset.NewSet(newReviewers...)

	// Find reviewers to add or remove
	toRemove := currentSet.Difference(newSet)
	toAdd := newSet.Difference(currentSet)

	// Remove old reviewers
	if toRemove.Cardinality() > 0 {
		if err = g.removeReviewers(ctx, repo, prNumber, toRemove.ToSlice()); err != nil {
			return err
		}
	}

	// Add new reviewers
	if toAdd.Cardinality() > 0 {
		if _, err = g.requestReviewers(ctx, repo, prNumber, github.ReviewersRequest{Reviewers: toAdd.ToSlice()}); err != nil {
			return err
		}
	}

	return nil
}

func (g *Github) mergePullRequest(ctx context.Context, repo string, prNumber int) error {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	_, _, err := g.client.PullRequests.Merge(ctx, g.project, repo, prNumber, "", nil)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, false); rateErr != nil {
			return fmt.Errorf("failed to merge pull request: %w: %w", rateErr, err)
		} else if !retry {
			return fmt.Errorf("failed to merge pull request: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if _, _, err = g.client.PullRequests.Merge(ctx, g.project, repo, prNumber, "", nil); err != nil {
			return fmt.Errorf("failed to merge pull request after retry: %w", err)
		}
	}

	return nil
}

func parsePR(resp *github.PullRequest) *scm.PullRequest {
	pr := &scm.PullRequest{
		ID:          int(resp.GetID()),
		Number:      resp.GetNumber(),
		Title:       resp.GetTitle(),
		Description: resp.GetBody(),
		Reviewers:   make([]string, 0, len(resp.RequestedReviewers)),
	}

	for _, reviewer := range resp.RequestedReviewers {
		pr.Reviewers = append(pr.Reviewers, reviewer.GetLogin())
	}

	return pr
}
