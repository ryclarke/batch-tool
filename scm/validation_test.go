package scm_test

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/scm"

	_ "github.com/ryclarke/batch-tool/scm/bitbucket"
	_ "github.com/ryclarke/batch-tool/scm/fake"
	_ "github.com/ryclarke/batch-tool/scm/github"
)

func TestProviderCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		teamOpts     *scm.PROptions
		draftOpts    *scm.PROptions
		wantTeamErr  bool
		wantDraftErr bool
	}{
		{
			name:         "github_supports_all",
			providerName: "github",
			teamOpts:     &scm.PROptions{TeamReviewers: []string{"team1"}},
			draftOpts:    &scm.PROptions{Draft: boolPtr(true)},
			wantTeamErr:  false,
			wantDraftErr: false,
		},
		{
			name:         "bitbucket_supports_none",
			providerName: "bitbucket",
			teamOpts:     &scm.PROptions{TeamReviewers: []string{"team1"}},
			draftOpts:    &scm.PROptions{Draft: boolPtr(true)},
			wantTeamErr:  true,
			wantDraftErr: true,
		},
		{
			name:         "fake_supports_all",
			providerName: "fake",
			teamOpts:     &scm.PROptions{TeamReviewers: []string{"team1"}},
			draftOpts:    &scm.PROptions{Draft: boolPtr(true)},
			wantTeamErr:  false,
			wantDraftErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := scm.Get(context.Background(), tt.providerName, "test-project")

			// Test team reviewer support
			err := provider.CheckCapabilities(tt.teamOpts)
			if (err != nil) != tt.wantTeamErr {
				t.Errorf("CheckCapabilities(team reviewers) error = %v, wantErr %v", err, tt.wantTeamErr)
			}

			// Test draft support
			err = provider.CheckCapabilities(tt.draftOpts)
			if (err != nil) != tt.wantDraftErr {
				t.Errorf("CheckCapabilities(draft) error = %v, wantErr %v", err, tt.wantDraftErr)
			}
		})
	}
}

func TestValidatePROptions(t *testing.T) {
	tests := []struct {
		name       string
		caps       *scm.Capabilities
		opts       *scm.PROptions
		wantErr    bool
		errMessage string
	}{
		{
			name: "nil_opts_no_error",
			caps: &scm.Capabilities{
				TeamReviewers: true,
				Draft:         true,
			},
			opts:    nil,
			wantErr: false,
		},
		{
			name: "supports_all_with_team_reviewers_ok",
			caps: &scm.Capabilities{
				TeamReviewers: true,
				Draft:         true,
			},
			opts: &scm.PROptions{
				TeamReviewers: []string{"org/team1", "org/team2"},
			},
			wantErr: false,
		},
		{
			name: "supports_all_with_draft_ok",
			caps: &scm.Capabilities{
				TeamReviewers: true,
				Draft:         true,
			},
			opts: &scm.PROptions{
				Draft: boolPtr(true),
			},
			wantErr: false,
		},
		{
			name: "no_support_with_team_reviewers_fails",
			caps: &scm.Capabilities{
				TeamReviewers: false,
				Draft:         false,
			},
			opts: &scm.PROptions{
				TeamReviewers: []string{"team1"},
			},
			wantErr:    true,
			errMessage: "does not support team reviewers",
		},
		{
			name: "no_support_with_draft_fails",
			caps: &scm.Capabilities{
				TeamReviewers: false,
				Draft:         false,
			},
			opts: &scm.PROptions{
				Draft: boolPtr(true),
			},
			wantErr:    true,
			errMessage: "does not support draft",
		},
		{
			name: "no_support_with_reviewers_ok",
			caps: &scm.Capabilities{
				TeamReviewers: false,
				Draft:         false,
			},
			opts: &scm.PROptions{
				Reviewers: []string{"user1", "user2"},
			},
			wantErr: false,
		},
		{
			name: "supports_all_with_all_options_ok",
			caps: &scm.Capabilities{
				TeamReviewers: true,
				Draft:         true,
			},
			opts: &scm.PROptions{
				TeamReviewers: []string{"team1"},
				Draft:         boolPtr(true),
				Title:         "Test PR",
				Reviewers:     []string{"user1"},
			},
			wantErr: false,
		},
		{
			name: "empty_team_reviewers_ok",
			caps: &scm.Capabilities{
				TeamReviewers: false,
				Draft:         false,
			},
			opts: &scm.PROptions{
				TeamReviewers: []string{},
			},
			wantErr: false,
		},
		{
			name:    "nil_caps_same_as_zero_value",
			caps:    nil,
			opts:    &scm.PROptions{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scm.ValidatePROptions(tt.caps, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePROptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMessage != "" {
				if !contains(err.Error(), tt.errMessage) {
					t.Errorf("ValidatePROptions() error message %q does not contain %q", err.Error(), tt.errMessage)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		(len(substr) > 0 && len(s) > len(substr)) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
