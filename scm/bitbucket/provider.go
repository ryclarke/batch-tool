package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

var _ scm.Provider = new(Bitbucket)

func init() {
	// Register the Bitbucket provider factory
	scm.Register("bitbucket", New)
}

// New creates a new Bitbucket SCM provider instance.
func New(ctx context.Context, project string) scm.Provider {
	viper := config.Viper(ctx)
	return &Bitbucket{
		client:  http.DefaultClient,
		scheme:  "https",
		host:    viper.GetString(config.GitHost),
		project: project,
		ctx:     ctx,
	}
}

// Bitbucket represents an SCM provider for the Bitbucket v1 API.
type Bitbucket struct {
	client  *http.Client
	scheme  string
	host    string
	project string
	ctx     context.Context
}

// constructs the base URL for the Bitbucket API endpoint.
func (b *Bitbucket) url(repo string, queryParams url.Values, path ...string) string {
	scheme := b.scheme
	if scheme == "" {
		scheme = "https"
	}
	// Create base URL using url.URL struct
	baseURL := &url.URL{
		Scheme: scheme,
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
	viper := config.Viper(b.ctx)
	req.Header.Set("Authorization", "Bearer "+viper.GetString(config.AuthToken))
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return doResp[T](b.client, req)
}

// doResp executes the HTTP request and unmarshals the response into the provided type.
func doResp[T any](client *http.Client, req *http.Request) (*T, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if err := parseError(resp); err != nil {
		return nil, err
	}

	var result T

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func parseError(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	output, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error %d: failed to read response body: %w", resp.StatusCode, err)
	}

	return fmt.Errorf("error %d: %s", resp.StatusCode, output)
}
