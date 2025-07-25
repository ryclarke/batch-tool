package bitbucket

import (
	"fmt"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/spf13/viper"
)

type repositoryListResp struct {
	Values []*scm.Repository `json:"values"`
}

// ListRepositories lists all repositories in the specified project.
func (b *Bitbucket) ListRepositories() ([]*scm.Repository, error) {
	resp, err := get[repositoryListResp](b, b.url("", "repos?limit=1000"))
	if err != nil {
		return nil, err
	}

	for _, repo := range resp.Values {
		repo.Project = b.project

		labels, err := b.getLabels(repo.Name)
		if err != nil {
			return nil, err
		}

		repo.Labels = labels

		defaultBranch, err := b.getDefaultBranch(repo.Name)
		if err != nil {
			// If we can't fetch the default branch, use the configured default
			defaultBranch = viper.GetString(config.SourceBranch)

			fmt.Printf("Error fetching default branch for %s - falling back on configured default '%s': %v\n", repo.Name, defaultBranch, err)
		}

		repo.DefaultBranch = defaultBranch
	}

	return resp.Values, nil
}

type labelListResp struct {
	Values []struct {
		Name string `json:"name"`
	} `json:"values"`
}

func (b *Bitbucket) getLabels(repo string) ([]string, error) {
	resp, err := get[labelListResp](b, b.url(repo, "labels?limit=100"))
	if err != nil {
		return nil, err
	}

	// Flatten the API response to extract the list of labels
	var labels = make([]string, len(resp.Values))
	for i, val := range resp.Values {
		labels[i] = val.Name
	}

	return labels, nil
}

type defaultBranchResp struct {
	DisplayId string `json:"displayId"`
}

func (b *Bitbucket) getDefaultBranch(repo string) (string, error) {
	resp, err := get[defaultBranchResp](b, b.url(repo, "default-branch"))
	if err != nil {
		return "", err
	}

	return resp.DisplayId, nil
}
