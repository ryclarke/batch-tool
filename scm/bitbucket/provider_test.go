package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/ryclarke/batch-tool/scm"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

func TestNew(t *testing.T) {
	ctx := loadFixture(t)
	provider := New(ctx, "test-project")

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	// Test that it implements the Provider interface
	var _ = provider
}

func TestBitbucketProviderCreation(t *testing.T) {
	// Test provider creation with different project names
	testCases := []string{
		"simple-project",
		"project-with-dashes",
		"project_with_underscores",
		"Project123",
	}

	for _, projectName := range testCases {
		t.Run("Project_"+projectName, func(t *testing.T) {
			ctx := loadFixture(t)
			provider := New(ctx, projectName)

			if provider == nil {
				t.Errorf("Expected non-nil provider for project %s", projectName)
			}

			// Verify it's the correct type
			bitbucketProvider, ok := provider.(*Bitbucket)
			if !ok {
				t.Errorf("Expected *Bitbucket provider, got %T", provider)
			}

			if bitbucketProvider.project != projectName {
				t.Errorf("Expected project %s, got %s", projectName, bitbucketProvider.project)
			}
		})
	}
}

func TestBitbucketProviderRegistration(t *testing.T) {
	ctx := loadFixture(t)
	// Test that the Bitbucket provider is registered during init
	provider := scm.Get(ctx, "bitbucket", "test-project")

	if provider == nil {
		t.Fatal("Expected Bitbucket provider to be registered")
	}

	// Verify it's the correct type
	_, ok := provider.(*Bitbucket)
	if !ok {
		t.Errorf("Expected *Bitbucket provider, got %T", provider)
	}
}

func TestBitbucketURL(t *testing.T) {
	bitbucket := &Bitbucket{
		host:    "bitbucket.example.com",
		project: "TEST",
	}

	tests := []struct {
		name        string
		repo        string
		queryParams url.Values
		path        []string
		expected    string
	}{
		{
			name:        "ProjectOnly",
			repo:        "",
			queryParams: nil,
			path:        nil,
			expected:    "https://bitbucket.example.com/rest/api/1.0/projects/TEST",
		},
		{
			name:        "ProjectWithRepo",
			repo:        "my-repo",
			queryParams: nil,
			path:        nil,
			expected:    "https://bitbucket.example.com/rest/api/1.0/projects/TEST/repos/my-repo",
		},
		{
			name:        "ProjectWithRepoAndPath",
			repo:        "my-repo",
			queryParams: nil,
			path:        []string{"pull-requests"},
			expected:    "https://bitbucket.example.com/rest/api/1.0/projects/TEST/repos/my-repo/pull-requests",
		},
		{
			name: "WithQueryParams",
			repo: "my-repo",
			queryParams: func() url.Values {
				params := url.Values{}
				params.Set("limit", "100")
				params.Set("direction", "outgoing")
				return params
			}(),
			path:     []string{"pull-requests"},
			expected: "https://bitbucket.example.com/rest/api/1.0/projects/TEST/repos/my-repo/pull-requests?direction=outgoing&limit=100",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := bitbucket.url(test.repo, test.queryParams, test.path...)
			if actual != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, actual)
			}
		})
	}
}

func TestBitbucketJSONParsing(t *testing.T) {
	// Test with realistic BitBucket API response structure
	repoListJSON := `{
		"size": 2,
		"limit": 1000,
		"isLastPage": true,
		"values": [
			{
				"slug": "test-repo",
				"id": 123456,
				"name": "test-repo",
				"description": "Test repository description",
				"hierarchyId": "abc123def456",
				"scmId": "git",
				"state": "AVAILABLE",
				"statusMessage": "Available",
				"forkable": true,
				"project": {
					"key": "TEST",
					"id": 5103,
					"name": "Test Project",
					"description": "Test Project Description",
					"public": false,
					"type": "NORMAL",
					"links": {
						"self": [
							{
								"href": "https://bitbucket.example.com/projects/TEST"
							}
						]
					}
				},
				"public": false,
				"links": {
					"clone": [
						{
							"href": "ssh://git@bitbucket.example.com/test/test-repo.git",
							"name": "ssh"
						},
						{
							"href": "https://bitbucket.example.com/scm/test/test-repo.git",
							"name": "http"
						}
					],
					"self": [
						{
							"href": "https://bitbucket.example.com/projects/TEST/repos/test-repo/browse"
						}
					]
				}
			},
			{
				"slug": "another-repo",
				"id": 789012,
				"name": "another-repo",
				"description": "Another test repository",
				"hierarchyId": "def456ghi789",
				"scmId": "git",
				"state": "AVAILABLE",
				"statusMessage": "Available",
				"forkable": true,
				"project": {
					"key": "TEST",
					"id": 5103,
					"name": "Test Project",
					"description": "Test Project Description",
					"public": false,
					"type": "NORMAL"
				},
				"public": true
			}
		],
		"start": 0
	}`

	// Test unmarshaling into our BitBucket-specific structs
	var resp repositoryListResp
	err := json.Unmarshal([]byte(repoListJSON), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal repository list response: %v", err)
	}

	// Validate the parsing worked correctly
	if len(resp.Values) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(resp.Values))
	}

	// Test first repository
	repo1 := resp.Values[0]
	if repo1.Name != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got '%s'", repo1.Name)
	}
	if repo1.Description != "Test repository description" {
		t.Errorf("Expected description 'Test repository description', got '%s'", repo1.Description)
	}
	if repo1.Public != false {
		t.Errorf("Expected public=false, got %v", repo1.Public)
	}
	if repo1.Project.Key != "TEST" {
		t.Errorf("Expected project key 'TEST', got '%s'", repo1.Project.Key)
	}
	if repo1.Project.Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got '%s'", repo1.Project.Name)
	}

	// Test second repository
	repo2 := resp.Values[1]
	if repo2.Name != "another-repo" {
		t.Errorf("Expected repo name 'another-repo', got '%s'", repo2.Name)
	}
	if repo2.Public != true {
		t.Errorf("Expected public=true, got %v", repo2.Public)
	}
}

func TestBitbucketLabelsJSONParsing(t *testing.T) {
	// Test with realistic BitBucket labels API response
	labelsJSON := `{
		"size": 3,
		"limit": 100,
		"isLastPage": true,
		"values": [
			{"name": "core"},
			{"name": "deprecated"},
			{"name": "testing"}
		],
		"start": 0
	}`

	var resp labelListResp
	err := json.Unmarshal([]byte(labelsJSON), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal labels response: %v", err)
	}

	if len(resp.Values) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(resp.Values))
	}

	expectedLabels := []string{"core", "deprecated", "testing"}
	for i, label := range resp.Values {
		if label.Name != expectedLabels[i] {
			t.Errorf("Expected label '%s', got '%s'", expectedLabels[i], label.Name)
		}
	}
}

func TestBitbucketDefaultBranchJSONParsing(t *testing.T) {
	// Test with realistic BitBucket default branch API response
	branchJSON := `{
		"id": "refs/heads/main",
		"displayId": "main",
		"type": "BRANCH",
		"latestCommit": "abc123def456",
		"latestChangeset": "abc123def456",
		"isDefault": true
	}`

	var resp defaultBranchResp
	err := json.Unmarshal([]byte(branchJSON), &resp)
	if err != nil {
		t.Fatalf("Failed to unmarshal default branch response: %v", err)
	}

	if resp.DisplayID != "main" {
		t.Errorf("Expected display ID 'main', got '%s'", resp.DisplayID)
	}
}
