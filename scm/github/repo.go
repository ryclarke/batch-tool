package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v74/github"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

// ListRepositories lists all repositories in the specified project.
// Supports both organization and user repositories.
func (g *Github) ListRepositories() ([]*scm.Repository, error) {
	output := make([]*scm.Repository, 0)
	opt := &github.RepositoryListByOrgOptions{
		Sort:        "full_name",
		ListOptions: github.ListOptions{PerPage: 20},
	}

	defer g.readLock()()

	for {
		repos, resp, err := g.listRepositories(context.TODO(), opt)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			if repo.GetDefaultBranch() == "" {
				// fall back on configured default branch if it isn't set for the repo
				defaultBranch := config.Viper(g.ctx).GetString(config.DefaultBranch)
				repo.DefaultBranch = &defaultBranch
			}

			output = append(output, &scm.Repository{
				Name:          repo.GetName(),
				Description:   repo.GetDescription(),
				Public:        !repo.GetPrivate(),
				Project:       g.project,
				DefaultBranch: repo.GetDefaultBranch(),
				Labels:        repo.Topics,
			})
		}

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return output, nil
}

func (g *Github) listRepositories(ctx context.Context, opt *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error) {
	// Try listing as organization first
	repos, resp, err := g.client.Repositories.ListByOrg(ctx, g.project, opt)
	if err != nil {
		// If we get a 404, the project might be a user, not an org
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// Convert options for user listing
			userOpt := &github.RepositoryListByUserOptions{
				Sort:        opt.Sort,
				ListOptions: opt.ListOptions,
			}
			return g.listUserRepositories(ctx, userOpt)
		}

		if retry, rateErr := g.handleRateLimitError(ctx, err, true); rateErr != nil {
			return nil, nil, fmt.Errorf("failed to list repositories: %w: %w", rateErr, err)
		} else if !retry {
			return nil, nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if repos, resp, err = g.client.Repositories.ListByOrg(ctx, g.project, opt); err != nil {
			// If we get a 404, the project might be a user, not an org
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				// Convert options for user listing
				userOpt := &github.RepositoryListByUserOptions{
					Sort:        opt.Sort,
					ListOptions: opt.ListOptions,
				}
				return g.listUserRepositories(ctx, userOpt)
			}

			return nil, nil, fmt.Errorf("failed to list repositories after retry: %w", err)
		}
	}

	return repos, resp, nil
}

func (g *Github) listUserRepositories(ctx context.Context, opt *github.RepositoryListByUserOptions) ([]*github.Repository, *github.Response, error) {
	repos, resp, err := g.client.Repositories.ListByUser(ctx, g.project, opt)
	if err != nil {
		if retry, rateErr := g.handleRateLimitError(ctx, err, true); rateErr != nil {
			return nil, nil, fmt.Errorf("failed to list user repositories: %w: %w", rateErr, err)
		} else if !retry {
			return nil, nil, fmt.Errorf("failed to list user repositories: %w", err)
		}

		// retry the request after waiting for the rate limit to reset
		if repos, resp, err = g.client.Repositories.ListByUser(ctx, g.project, opt); err != nil {
			return nil, nil, fmt.Errorf("failed to list user repositories after retry: %w", err)
		}
	}

	return repos, resp, nil
}
