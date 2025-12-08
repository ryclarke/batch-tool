package bitbucket

import (
	"fmt"
	"net/url"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

// BitBucket-specific structs to handle API response format
type bitbucketProject struct {
	Key         string `json:"key"`
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Type        string `json:"type"`
}

type bitbucketRepository struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Public      bool             `json:"public"`
	Project     bitbucketProject `json:"project"`
	// Note: BitBucket doesn't have a default_branch field in the repo list API
	// We'll fetch it separately
}

type repositoryListResp struct {
	Values []*bitbucketRepository `json:"values"`
}

// ListRepositories lists all repositories in the specified project.
func (b *Bitbucket) ListRepositories() ([]*scm.Repository, error) {
	queryParams := url.Values{}
	queryParams.Set("limit", "1000")
	resp, err := get[repositoryListResp](b, b.url("", queryParams, "repos"))
	if err != nil {
		return nil, err
	}

	// Convert BitBucket-specific format to standard Repository format
	repositories := make([]*scm.Repository, len(resp.Values))
	for i, bbRepo := range resp.Values {
		// Create standard Repository struct
		repo := &scm.Repository{
			Name:        bbRepo.Name,
			Description: bbRepo.Description,
			Public:      bbRepo.Public,
			Project:     bbRepo.Project.Key, // Extract the key from the project object
		}

		// Get labels for this repository
		labels, err := b.getLabels(repo.Name)
		if err != nil {
			return nil, err
		}
		repo.Labels = labels

		// Get default branch for this repository
		defaultBranch, err := b.getDefaultBranch(repo.Name)
		if err != nil {
			// If we can't fetch the default branch, use the configured default
			viper := config.Viper(b.ctx)
			defaultBranch = viper.GetString(config.SourceBranch)
			fmt.Printf("Error fetching default branch for %s - falling back on configured default '%s': %v\n", repo.Name, defaultBranch, err)
		}
		repo.DefaultBranch = defaultBranch

		repositories[i] = repo
	}

	return repositories, nil
}

type labelListResp struct {
	Values []struct {
		Name string `json:"name"`
	} `json:"values"`
}

func (b *Bitbucket) getLabels(repo string) ([]string, error) {
	queryParams := url.Values{}
	queryParams.Set("limit", "100")
	resp, err := get[labelListResp](b, b.url(repo, queryParams, "labels"))
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
	resp, err := get[defaultBranchResp](b, b.url(repo, nil, "default-branch"))
	if err != nil {
		return "", err
	}

	return resp.DisplayId, nil
}
