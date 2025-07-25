package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v74/github"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

// ListRepositories lists all repositories in the specified project.
func (g *Github) ListRepositories() ([]*scm.Repository, error) {
	output := make([]*scm.Repository, 0)
	opt := &github.RepositoryListByOrgOptions{
		Sort:        "full_name",
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for {
		repos, resp, err := g.client.Repositories.ListByOrg(context.TODO(), g.project, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		for _, repo := range repos {
			if repo.GetDefaultBranch() == "" {
				// fall back on configured default branch if it isn't set for the repo
				defaultBranch := viper.GetString(config.SourceBranch)
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
