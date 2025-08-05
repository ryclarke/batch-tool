package bitbucket

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/spf13/viper"
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
func (b *Bitbucket) url(repo string, path ...string) string {
	uri := url.PathEscape(fmt.Sprintf("https://%s/rest/api/1.0/projects/%s", b.host, b.project))

	// If a repository is specified, append it to the URL.
	if repo != "" {
		uri = fmt.Sprintf("%s/repos/%s", uri, url.PathEscape(repo))
	}

	// If additional path segments are provided, append them to the URL.
	if len(path) > 0 {
		uri = fmt.Sprintf("%s/%s", uri, url.PathEscape(path[0]))
	}

	return uri
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
