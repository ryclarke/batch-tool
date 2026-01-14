package github

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/go-github/v74/github"

	"github.com/ryclarke/batch-tool/scm"
)

// applyReviewers applies the specified reviewers and team reviewers to the given pull request based on the provided options.
func (g *Github) applyReviewers(repo string, pr *github.PullRequest, opts *scm.PROptions) (*github.PullRequest, error) {
	if opts == nil {
		opts = &scm.PROptions{}
	}

	var err error

	if len(opts.Reviewers) > 0 {
		// If ResetReviewers is true, replace existing reviewers with the provided list (default behavior is to append)
		if opts.ResetReviewers {
			if pr, err = g.replaceReviewers(repo, pr.GetNumber(), opts.Reviewers); err != nil {
				return nil, err
			}
		} else {
			// GitHub's RequestReviewers API appends to existing reviewers
			if pr, err = g.requestReviewers(repo, pr.GetNumber(), opts.Reviewers); err != nil {
				return nil, err
			}
		}
	}

	if len(opts.TeamReviewers) > 0 {
		// If ResetReviewers is true, replace existing team reviewers with the provided list (default behavior is to append)
		if opts.ResetReviewers {
			if pr, err = g.replaceTeamReviewers(repo, pr.GetNumber(), opts.TeamReviewers); err != nil {
				return nil, err
			}
		} else {
			// GitHub's RequestTeamReviewers API appends to existing team reviewers
			if pr, err = g.requestTeamReviewers(repo, pr.GetNumber(), opts.TeamReviewers); err != nil {
				return nil, err
			}
		}
	}

	return pr, nil
}

// requestReviewers requests the specified reviewers for the given pull request.
func (g *Github) requestReviewers(repo string, prNumber int, reviewers []string) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	req := github.ReviewersRequest{Reviewers: reviewers}

	resp, _, err := g.client.PullRequests.RequestReviewers(g.ctx, g.project, repo, prNumber, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to request reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to request reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.RequestReviewers(g.ctx, g.project, repo, prNumber, req); err != nil {
			return nil, fmt.Errorf("failed to request reviewers after retry: %w", err)
		}
	}

	return resp, nil
}

// replaceReviewers replaces the current reviewers with the provided list
func (g *Github) replaceReviewers(repo string, prNumber int, newReviewers []string) (*github.PullRequest, error) {
	// Get current reviewers
	currentReviewers, err := g.listReviewers(repo, prNumber)
	if err != nil {
		return nil, err
	}

	// Build sets for comparison
	currentSet := mapset.NewSet(currentReviewers...)
	newSet := mapset.NewSet(newReviewers...)

	// Find reviewers to add or remove
	toRemove := currentSet.Difference(newSet)
	toAdd := newSet.Difference(currentSet)

	// Remove old reviewers
	if toRemove.Cardinality() > 0 {
		if err = g.removeReviewers(repo, prNumber, toRemove.ToSlice()); err != nil {
			return nil, err
		}
	}

	// Add new reviewers
	if toAdd.Cardinality() > 0 {
		if _, err = g.requestReviewers(repo, prNumber, toAdd.ToSlice()); err != nil {
			return nil, err
		}
	}

	// Refresh PR to get updated reviewer list
	return g.getPullRequestByNumber(repo, prNumber)
}

// listReviewers returns a list of usernames of the reviewers for the given pull request.
func (g *Github) listReviewers(repo string, prNumber int) ([]string, error) {
	// acquire read lock (and release it when done)
	defer g.readLock()()

	reviewers, _, err := g.client.PullRequests.ListReviewers(g.ctx, g.project, repo, prNumber, nil)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, true); rateErr != nil {
			return nil, fmt.Errorf("failed to list reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to list reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if reviewers, _, err = g.client.PullRequests.ListReviewers(g.ctx, g.project, repo, prNumber, nil); err != nil {
			return nil, fmt.Errorf("failed to list reviewers after retry: %w", err)
		}
	}

	result := make([]string, 0, len(reviewers.Users))
	for _, user := range reviewers.Users {
		result = append(result, user.GetLogin())
	}

	return result, nil
}

// removeReviewers removes the specified reviewers from the given pull request.
func (g *Github) removeReviewers(repo string, prNumber int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	// acquire write lock (and release it when done)
	defer g.writeLock()()

	_, err := g.client.PullRequests.RemoveReviewers(g.ctx, g.project, repo, prNumber, github.ReviewersRequest{Reviewers: reviewers})
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return fmt.Errorf("failed to remove reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return fmt.Errorf("failed to remove reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if _, err = g.client.PullRequests.RemoveReviewers(g.ctx, g.project, repo, prNumber, github.ReviewersRequest{Reviewers: reviewers}); err != nil {
			return fmt.Errorf("failed to remove reviewers after retry: %w", err)
		}
	}

	return nil
}

// requestTeamReviewers requests the specified team reviewers for the given pull request.
func (g *Github) requestTeamReviewers(repo string, prNumber int, teamReviewers []string) (*github.PullRequest, error) {
	// acquire write lock (and release it when done)
	defer g.writeLock()()

	req := github.ReviewersRequest{TeamReviewers: teamReviewers}

	resp, _, err := g.client.PullRequests.RequestReviewers(g.ctx, g.project, repo, prNumber, req)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return nil, fmt.Errorf("failed to request team reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to request team reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if resp, _, err = g.client.PullRequests.RequestReviewers(g.ctx, g.project, repo, prNumber, req); err != nil {
			return nil, fmt.Errorf("failed to request team reviewers after retry: %w", err)
		}
	}

	return resp, nil
}

// replaceTeamReviewers replaces the current team reviewers with the provided list
func (g *Github) replaceTeamReviewers(repo string, prNumber int, newTeamReviewers []string) (*github.PullRequest, error) {
	// Get current team reviewers
	currentTeamReviewers, err := g.listTeamReviewers(repo, prNumber)
	if err != nil {
		return nil, err
	}

	// Build sets for comparison
	currentSet := mapset.NewSet(currentTeamReviewers...)
	newSet := mapset.NewSet(newTeamReviewers...)

	// Find team reviewers to add or remove
	toRemove := currentSet.Difference(newSet)
	toAdd := newSet.Difference(currentSet)

	// Remove old team reviewers
	if toRemove.Cardinality() > 0 {
		if err = g.removeTeamReviewers(repo, prNumber, toRemove.ToSlice()); err != nil {
			return nil, err
		}
	}

	// Add new team reviewers
	if toAdd.Cardinality() > 0 {
		if _, err = g.requestTeamReviewers(repo, prNumber, toAdd.ToSlice()); err != nil {
			return nil, err
		}
	}

	// Refresh PR to get updated reviewer list
	return g.getPullRequestByNumber(repo, prNumber)
}

// listTeamReviewers returns a list of team reviewer slugs (in format "org/team-slug")
func (g *Github) listTeamReviewers(repo string, prNumber int) ([]string, error) {
	// acquire read lock (and release it when done)
	defer g.readLock()()

	reviewers, _, err := g.client.PullRequests.ListReviewers(g.ctx, g.project, repo, prNumber, nil)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, true); rateErr != nil {
			return nil, fmt.Errorf("failed to list team reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return nil, fmt.Errorf("failed to list team reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if reviewers, _, err = g.client.PullRequests.ListReviewers(g.ctx, g.project, repo, prNumber, nil); err != nil {
			return nil, fmt.Errorf("failed to list team reviewers after retry: %w", err)
		}
	}

	result := make([]string, 0, len(reviewers.Teams))
	for _, team := range reviewers.Teams {
		if team.Organization != nil && team.Slug != nil {
			result = append(result, fmt.Sprintf("%s/%s", team.Organization.GetLogin(), team.GetSlug()))
		}
	}

	return result, nil
}

// removeTeamReviewers removes the specified team reviewers from the given pull request.
func (g *Github) removeTeamReviewers(repo string, prNumber int, teamReviewers []string) error {
	if len(teamReviewers) == 0 {
		return nil
	}

	// acquire write lock (and release it when done)
	defer g.writeLock()()

	_, err := g.client.PullRequests.RemoveReviewers(g.ctx, g.project, repo, prNumber, github.ReviewersRequest{TeamReviewers: teamReviewers})
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(err, false); rateErr != nil {
			return fmt.Errorf("failed to remove team reviewers: %w: %w", rateErr, err)
		} else if !retry {
			return fmt.Errorf("failed to remove team reviewers: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if _, err = g.client.PullRequests.RemoveReviewers(g.ctx, g.project, repo, prNumber, github.ReviewersRequest{TeamReviewers: teamReviewers}); err != nil {
			return fmt.Errorf("failed to remove team reviewers after retry: %w", err)
		}
	}

	return nil
}
