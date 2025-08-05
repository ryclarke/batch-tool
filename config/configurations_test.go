package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestInit(t *testing.T) {
	// Save original viper state
	originalConfig := viper.AllSettings()
	defer func() {
		viper.Reset()
		for key, value := range originalConfig {
			viper.Set(key, value)
		}
	}()

	// Reset viper to test initialization
	viper.Reset()

	// Call Init
	Init()

	// Test default values
	testCases := []struct {
		key      string
		expected interface{}
	}{
		{GitUser, "git"},
		{GitHost, "github.com"},
		{GitProvider, "github"},
		{SourceBranch, "main"},
		{SortRepos, true},
		{SkipUnwanted, true},
		{UseSync, false},
		{CatalogCacheFile, ".catalog"},
		{CatalogCacheTTL, "24h"},
		{ChannelBuffer, 100},
	}

	for _, tc := range testCases {
		actual := viper.Get(tc.key)
		if actual != tc.expected {
			t.Errorf("Expected %s to be %v, got %v", tc.key, tc.expected, actual)
		}
	}

	// Test UnwantedLabels default
	unwantedLabels := viper.GetStringSlice(UnwantedLabels)
	expectedLabels := []string{"deprecated", "poc"}
	if len(unwantedLabels) != len(expectedLabels) {
		t.Errorf("Expected %d unwanted labels, got %d", len(expectedLabels), len(unwantedLabels))
	}
	for i, label := range expectedLabels {
		if i >= len(unwantedLabels) || unwantedLabels[i] != label {
			t.Errorf("Expected unwanted label %s at index %d, got %v", label, i, unwantedLabels)
		}
	}

	// Test that DefaultReviewers is initialized as empty map
	defaultReviewers := viper.GetStringMapStringSlice(DefaultReviewers)
	if defaultReviewers == nil {
		t.Error("Expected DefaultReviewers to be initialized as empty map")
	}

	// Test that RepoAliases is initialized as empty map
	repoAliases := viper.GetStringMapStringSlice(RepoAliases)
	if repoAliases == nil {
		t.Error("Expected RepoAliases to be initialized as empty map")
	}
}

func TestInitWithConfigFile(t *testing.T) {
	// Save original viper state
	originalConfig := viper.AllSettings()
	defer func() {
		viper.Reset()
		for key, value := range originalConfig {
			viper.Set(key, value)
		}
	}()

	// Test with explicit config file
	CfgFile = "nonexistent-config.yaml"
	defer func() { CfgFile = "" }()

	viper.Reset()
	Init()

	// Should still set defaults even if config file doesn't exist
	if viper.GetString(GitHost) != "github.com" {
		t.Error("Expected defaults to be set even with nonexistent config file")
	}
}

func TestInitEnvironmentVariables(t *testing.T) {
	// Save original viper state
	originalConfig := viper.AllSettings()
	defer func() {
		viper.Reset()
		for key, value := range originalConfig {
			viper.Set(key, value)
		}
	}()

	// Test environment variable replacement
	viper.Reset()

	// Set an environment variable with underscores (viper should convert dots/hyphens)
	os.Setenv("GIT_HOST", "custom.example.com")
	defer os.Unsetenv("GIT_HOST")

	Init()

	// The environment variable should override the default
	if viper.GetString(GitHost) != "custom.example.com" {
		t.Errorf("Expected environment variable to override default, got %s", viper.GetString(GitHost))
	}
}

func TestConstants(t *testing.T) {
	// Test that all expected constants are defined
	constants := map[string]string{
		"Version":          Version,
		"GitUser":          GitUser,
		"GitHost":          GitHost,
		"GitProject":       GitProject,
		"GitProvider":      GitProvider,
		"SourceBranch":     SourceBranch,
		"Branch":           Branch,
		"SortRepos":        SortRepos,
		"SkipUnwanted":     SkipUnwanted,
		"UnwantedLabels":   UnwantedLabels,
		"RepoAliases":      RepoAliases,
		"DefaultReviewers": DefaultReviewers,
		"CatalogCacheFile": CatalogCacheFile,
		"CatalogCacheTTL":  CatalogCacheTTL,
		"EnvGopath":        EnvGopath,
		"CommitMessage":    CommitMessage,
		"CommitAmend":      CommitAmend,
		"Reviewers":        Reviewers,
		"AuthToken":        AuthToken,
		"UseSync":          UseSync,
		"ChannelBuffer":    ChannelBuffer,
	}

	for name, value := range constants {
		if value == "" {
			t.Errorf("Constant %s is empty", name)
		}
	}

	// Test URL templates are defined
	if ApiPathTmpl == "" {
		t.Error("ApiPathTmpl is empty")
	}
	if PrTmpl == "" {
		t.Error("PrTmpl is empty")
	}
	if PrRepoTmpl == "" {
		t.Error("PrRepoTmpl is empty")
	}
	if PrReviewerTmpl == "" {
		t.Error("PrReviewerTmpl is empty")
	}
}

func TestTemplateFormats(t *testing.T) {
	// Test that template strings contain expected format specifiers
	if !containsFormatSpecifier(ApiPathTmpl, "%s") {
		t.Errorf("ApiPathTmpl should contain %%s format specifiers")
	}

	if !containsFormatSpecifier(PrTmpl, "%s") {
		t.Errorf("PrTmpl should contain %%s format specifiers")
	}

	if !containsFormatSpecifier(PrRepoTmpl, "%s") {
		t.Errorf("PrRepoTmpl should contain %%s format specifiers")
	}

	if !containsFormatSpecifier(PrReviewerTmpl, "%s") {
		t.Errorf("PrReviewerTmpl should contain %%s format specifiers")
	}
}

// Helper function to check if a string contains format specifiers
func containsFormatSpecifier(s, spec string) bool {
	count := 0
	for i := 0; i < len(s)-1; i++ {
		if s[i:i+2] == spec {
			count++
		}
	}
	return count > 0
}
