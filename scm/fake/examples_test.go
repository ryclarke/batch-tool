// Package fake provides examples and tests for the fake SCM provider.
package fake

import (
	"context"
	"fmt"
	"testing"

	"github.com/ryclarke/batch-tool/scm"
)

// ExampleFake_basicUsage demonstrates basic usage of the fake SCM provider
func ExampleFake_basicUsage() {
	ctx := context.Background()
	// Create a new fake provider
	fake := New(ctx, "my-project").(*Fake)

	// Add some test repositories
	fake.AddRepository(&scm.Repository{
		Name:        "repo-1",
		Description: "First repository",
		Project:     "my-project",
		Labels:      []string{"backend", "go"},
	})

	fake.AddRepository(&scm.Repository{
		Name:        "repo-2",
		Description: "Second repository",
		Project:     "my-project",
		Labels:      []string{"frontend", "javascript"},
	})

	// List repositories
	repos, _ := fake.ListRepositories()
	println("Found", len(repos), "repositories")

	// Create a pull request
	pr, _ := fake.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "My Feature", Description: "Description", Reviewers: []string{"reviewer1"}})
	println("Created PR with ID:", pr.ID)
}

// ExampleCreateTestRepositories demonstrates using the CreateTestRepositories helper
func ExampleCreateTestRepositories() {
	// Create a fake provider with pre-built test data
	testRepos := CreateTestRepositories("test-project")
	fake := NewFake("test-project", testRepos)

	// Get repositories by label
	activeRepos := fake.GetRepositoriesByLabel("active")
	println("Found", len(activeRepos), "active repositories")

	// Get all available labels
	labels := fake.GetAllLabels()
	println("Available labels:", len(labels))
}

// ExampleFake_errorTesting demonstrates how to configure errors for testing
func ExampleFake_errorTesting() {
	ctx := context.Background()
	fake := New(ctx, "test-project").(*Fake)

	// Configure the provider to return an error
	fake.SetError("ListRepositories", fmt.Errorf("simulated API error"))

	// Now ListRepositories will return the configured error
	_, err := fake.ListRepositories()
	if err != nil {
		println("Got expected error:", err.Error())
	}

	// Clear the error
	fake.ClearError("ListRepositories")

	// Now it works normally again
	repos, err := fake.ListRepositories()
	if err == nil {
		println("No error, found", len(repos), "repositories")
	}
}

// TestExampleIntegration shows how to use the fake provider in integration tests
func TestExampleIntegration(t *testing.T) {
	// This is how you would use the fake provider in your actual tests

	// Setup: Create fake provider with test data
	fake := NewFake("test-project", CreateTestRepositories("test-project"))
	repos, err := fake.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	if len(repos) != 5 {
		t.Errorf("Expected 5 repositories, got %d", len(repos))
	}

	// Test scenario: Create and retrieve pull request
	originalPR, err := fake.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test Feature", Description: "Test Description", Reviewers: []string{"alice", "bob"}})
	if err != nil {
		t.Fatalf("Failed to create pull request: %v", err)
	}

	retrievedPR, err := fake.GetPullRequest("repo-1", "feature-branch")
	if err != nil {
		t.Fatalf("Failed to retrieve pull request: %v", err)
	}

	if retrievedPR.ID != originalPR.ID {
		t.Errorf("PR ID mismatch: expected %d, got %d", originalPR.ID, retrievedPR.ID)
	}

	// Test scenario: Update pull request (ResetReviewers: false means append)
	updatedPR, err := fake.UpdatePullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Updated Feature", Description: "Updated Description", Reviewers: []string{"charlie"}, ResetReviewers: false})
	if err != nil {
		t.Fatalf("Failed to update pull request: %v", err)
	}

	expectedReviewers := 3 // alice, bob, charlie
	if len(updatedPR.Reviewers) != expectedReviewers {
		t.Errorf("Expected %d reviewers after update, got %d", expectedReviewers, len(updatedPR.Reviewers))
	}

	// Test scenario: Merge pull request
	mergedPR, err := fake.MergePullRequest("repo-1", "feature-branch", false)
	if err != nil {
		t.Fatalf("Failed to merge pull request: %v", err)
	}

	if _, ok := fake.PullRequests[mergedPR.Repo]; ok {
		t.Error("Expected PR state to be MERGED, but it's still open")
	}
}

// TestErrorScenarios demonstrates testing error conditions
func TestErrorScenarios(t *testing.T) {
	fake := NewFake("test-project", CreateTestRepositories("test-project"))

	// Test API error simulation
	fake.SetError("ListRepositories", fmt.Errorf("API unavailable"))
	_, err := fake.ListRepositories()
	if err == nil {
		t.Error("Expected error when API is unavailable")
	}
	fake.ClearError("ListRepositories")

	// Test nonexistent resource errors
	_, err = fake.GetPullRequest("nonexistent-repo", "branch")
	if err == nil {
		t.Error("Expected error when getting nonexistent pull request")
	}

	// Test duplicate resource errors
	_, err = fake.OpenPullRequest("repo-1", "branch-1", &scm.PROptions{Title: "PR 1", Description: "Description", Reviewers: []string{}})
	if err != nil {
		t.Fatalf("Failed to create first PR: %v", err)
	}

	_, err = fake.OpenPullRequest("repo-1", "branch-1", &scm.PROptions{Title: "PR 2", Description: "Description", Reviewers: []string{}})
	if err == nil {
		t.Error("Expected error when creating duplicate pull request")
	}
}

// TestDataIsolation verifies that test data is properly isolated between tests
func TestDataIsolation(t *testing.T) {
	// Create two separate fake providers
	fake1 := NewFake("project-1", CreateTestRepositories("project-1"))
	fake2 := NewFake("project-2", CreateTestRepositories("project-2"))

	// Add data to fake1
	_, err := fake1.OpenPullRequest("repo-1", "branch-1", &scm.PROptions{Title: "PR in fake1", Description: "Description", Reviewers: []string{}})
	if err != nil {
		t.Fatalf("Failed to create PR in fake1: %v", err)
	}

	// fake2 should not have the PR from fake1
	if fake2.HasPullRequest("repo-1", "branch-1") {
		t.Error("fake2 should not have PR from fake1")
	}

	// Modify repository data in fake1
	fake1.AddRepository(&scm.Repository{
		Name:    "extra-repo",
		Project: "project-1",
		Labels:  []string{"extra"},
	})

	// fake2 should not see the extra repository
	if fake2.GetRepositoryByName("extra-repo") != nil {
		t.Error("fake2 should not see extra repository from fake1")
	}

	// Verify each fake has the correct number of repositories
	repos1, _ := fake1.ListRepositories()
	repos2, _ := fake2.ListRepositories()

	if len(repos1) != 6 { // 5 original + 1 extra
		t.Errorf("fake1 should have 6 repositories, got %d", len(repos1))
	}

	if len(repos2) != 5 { // 5 original only
		t.Errorf("fake2 should have 5 repositories, got %d", len(repos2))
	}
}
