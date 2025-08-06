package bitbucket

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

var _ scm.Provider = new(Bitbucket)

func init() {
	// Register the Bitbucket provider factory
	scm.Register("bitbucket", New)
}

// New creates a new Bitbucket SCM provider instance.
func New(project string) scm.Provider {
	return &Bitbucket{
		client:  http.DefaultClient,
		host:    viper.GetString(config.GitHost),
		project: project,
	}
}

// Bitbucket represents an SCM provider for the Bitbucket v1 API.
type Bitbucket struct {
	client  *http.Client
	host    string
	project string
}

// constructs the base URL for the Bitbucket API endpoint.
func (b *Bitbucket) url(repo string, queryParams url.Values, path ...string) string {
	// Create base URL using url.URL struct
	baseURL := &url.URL{
		Scheme: "https",
		Host:   b.host,
		Path:   "/rest/api/1.0/projects",
	}
	baseURL = baseURL.JoinPath(b.project)

	// Build dynamic path segments if needed
	if repo != "" {
		baseURL = baseURL.JoinPath("repos", repo)
	}

	baseURL = baseURL.JoinPath(path...)

	// Add query parameters if provided
	if queryParams != nil {
		baseURL.RawQuery = queryParams.Encode()
	}

	return baseURL.String()
}

// convenience function to perform a GET request and unmarshal the response into the specified type.
func get[T any](b *Bitbucket, path string) (*T, error) {
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return do[T](b, req)
}

// convenience function to perform an HTTP request and unmarshal the response into the specified type.
func do[T any](b *Bitbucket, req *http.Request) (*T, error) {
	req.Header.Set("Authorization", "Bearer "+viper.GetString(config.AuthToken))
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return scm.DoResp[T](b.client, req)
}
