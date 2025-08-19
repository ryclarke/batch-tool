package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v74/github"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
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
func (g *Github) OpenPullRequest(repo, branch, title, description string, reviewers []string) (*scm.PullRequest, error) {
	// reads are less restrictive than a failed write, so check for existing PR first
	if _, err := g.getPullRequest(context.TODO(), repo, branch); err == nil {
		return nil, fmt.Errorf("a pull request already exists for branch %s in repository %s", branch, repo)
	}

	// check default branch for the current repo, or use the fallback config
	defaultBranch := catalog.Catalog[repo].DefaultBranch
	if defaultBranch == "" {
		defaultBranch = viper.GetString(config.SourceBranch)
	}

	// if title is not specified, use the branch name
	if title == "" {
		title = branch
	}

	req := &github.NewPullRequest{
		Title: github.Ptr(title),
		Body:  github.Ptr(description),
		Head:  github.Ptr(branch),
		Base:  github.Ptr(defaultBranch),
	}

	resp, err := g.openPullRequest(context.TODO(), repo, req)
	if err != nil {
		return nil, err
	}

	if len(reviewers) > 0 {
		if resp, err = g.requestReviewers(context.TODO(), repo, resp.GetNumber(), github.ReviewersRequest{Reviewers: reviewers}); err != nil {
			return nil, err
		}
	}

	return parsePR(resp), nil
}

// UpdatePullRequest updates an existing pull request.
func (g *Github) UpdatePullRequest(repo, branch, title, description string, reviewers []string, appendReviewers bool) (*scm.PullRequest, error) {
	pr, err := g.getPullRequest(context.TODO(), repo, branch)
	if err != nil {
		return nil, err
	}

	if title != "" || description != "" {
		req := &github.PullRequest{
			Title: &title,
			Body:  &description,
		}

		if pr, err = g.editPullRequest(context.TODO(), repo, pr.GetNumber(), req); err != nil {
			return nil, err
		}
	}

	if len(reviewers) > 0 {
		if pr, err = g.requestReviewers(context.TODO(), repo, pr.GetNumber(), github.ReviewersRequest{Reviewers: reviewers}); err != nil {
			return nil, err
		}
	}

	return parsePR(pr), nil
}

func (g *Github) MergePullRequest(repo, branch string) (*scm.PullRequest, error) {
	pr, err := g.getPullRequest(context.TODO(), repo, branch)
	if err != nil {
		return nil, err
	}

	if !pr.GetMergeable() {
		return nil, fmt.Errorf("pull request %s [%d] for %s is not mergeable: %s", branch, pr.GetNumber(), repo, pr.GetMergeableState())
	}

	if err = g.mergePullRequest(context.TODO(), repo, pr.GetNumber()); err != nil {
		return nil, err
	}

	return parsePR(pr), nil
}

func (g *Github) getPullRequest(ctx context.Context, repo, branch string) (*github.PullRequest, error) {
	// acquire read lock (and release it when done)
	defer readLock()()

	opts := &github.PullRequestListOptions{
		State: "open",
		Head:  fmt.Sprintf("%s:%s", g.project, branch),
	}

	resp, _, err := g.client.PullRequests.List(ctx, g.project, repo, opts)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); !ok {
			return nil, fmt.Errorf("failed to get pull request: %w", err)
		} else {
			if rateErr := g.waitForRateLimit(ctx, true); rateErr != nil {
				return nil, fmt.Errorf("failed to get pull request: %w: %w", rateErr, err)
			}

			// retry the request after waiting for the rate limit to reset
			resp, _, err = g.client.PullRequests.List(ctx, g.project, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to get pull request after retry: %w", err)
			}
		}
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("no open pull request found for branch %s in repository %s", branch, repo)
	}

	return resp[0], nil
}

func (g *Github) openPullRequest(ctx context.Context, repo string, req *github.NewPullRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer writeLock()()

	resp, _, err := g.client.PullRequests.Create(ctx, g.project, repo, req)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); !ok {
			return nil, fmt.Errorf("failed to open pull request: %w", err)
		} else {
			if rateErr := g.waitForRateLimit(ctx, false); rateErr != nil {
				return nil, fmt.Errorf("failed to open pull request: %w: %w", rateErr, err)
			}

			// retry the request after waiting for the rate limit to reset
			resp, _, err = g.client.PullRequests.Create(ctx, g.project, repo, req)
			if err != nil {
				return nil, fmt.Errorf("failed to open pull request after retry: %w", err)
			}
		}
	}

	return resp, nil
}

func (g *Github) editPullRequest(ctx context.Context, repo string, prNumber int, req *github.PullRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer writeLock()()

	pr, _, err := g.client.PullRequests.Edit(ctx, g.project, repo, prNumber, req)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); !ok {
			return nil, fmt.Errorf("failed to update pull request: %w", err)
		} else {
			if rateErr := g.waitForRateLimit(ctx, false); rateErr != nil {
				return nil, fmt.Errorf("failed to update pull request: %w: %w", rateErr, err)
			}

			// retry the request after waiting for the rate limit to reset
			if pr, _, err = g.client.PullRequests.Edit(ctx, g.project, repo, prNumber, req); err != nil {
				return nil, fmt.Errorf("failed to update pull request after retry: %w", err)
			}
		}
	}

	return pr, nil
}

func (g *Github) requestReviewers(ctx context.Context, repo string, prNumber int, req github.ReviewersRequest) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer writeLock()()

	resp, _, err := g.client.PullRequests.RequestReviewers(ctx, g.project, repo, prNumber, req)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); !ok {
			return nil, fmt.Errorf("failed to request reviewers: %w", err)
		} else {
			if rateErr := g.waitForRateLimit(ctx, false); rateErr != nil {
				return nil, fmt.Errorf("failed to request reviewers: %w: %w", rateErr, err)
			}

			// retry the request after waiting for the rate limit to reset
			if resp, _, err = g.client.PullRequests.RequestReviewers(ctx, g.project, repo, prNumber, req); err != nil {
				return nil, fmt.Errorf("failed to request reviewers after retry: %w", err)
			}
		}
	}

	return resp, nil
}

func (g *Github) mergePullRequest(ctx context.Context, repo string, prNumber int) error {
	// acquire write lock (and release it when done)
	defer writeLock()()

	_, _, err := g.client.PullRequests.Merge(ctx, g.project, repo, prNumber, "", nil)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); !ok {
			return fmt.Errorf("failed to merge pull request: %w", err)
		} else {
			if rateErr := g.waitForRateLimit(ctx, false); rateErr != nil {
				return fmt.Errorf("failed to merge pull request: %w: %w", rateErr, err)
			}

			// retry the request after waiting for the rate limit to reset
			if _, _, err = g.client.PullRequests.Merge(ctx, g.project, repo, prNumber, "", nil); err != nil {
				return fmt.Errorf("failed to merge pull request after retry: %w", err)
			}
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
