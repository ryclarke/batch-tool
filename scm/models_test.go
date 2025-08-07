package scm

import (
	"testing"
)

func TestRepositoryStruct(t *testing.T) {
	repo := Repository{
		Name:          "test-repo",
		Description:   "A test repository",
		Public:        true,
		Project:       "test-project",
		DefaultBranch: "main",
		Labels:        []string{"backend", "go", "testing"},
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got %s", repo.Name)
	}

	if repo.Description != "A test repository" {
		t.Errorf("Expected description 'A test repository', got %s", repo.Description)
	}

	if !repo.Public {
		t.Error("Expected repository to be public")
	}

	if repo.Project != "test-project" {
		t.Errorf("Expected project 'test-project', got %s", repo.Project)
	}

	if repo.DefaultBranch != "main" {
		t.Errorf("Expected default branch 'main', got %s", repo.DefaultBranch)
	}

	if len(repo.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(repo.Labels))
	}

	expectedLabels := []string{"backend", "go", "testing"}
	for i, label := range repo.Labels {
		if label != expectedLabels[i] {
			t.Errorf("Expected label %s at position %d, got %s", expectedLabels[i], i, label)
		}
	}
}
