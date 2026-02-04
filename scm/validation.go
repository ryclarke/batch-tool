package scm

import (
	"fmt"
)

// Capabilities defines which PR options are supported by a provider.
type Capabilities struct {
	TeamReviewers  bool
	ResetReviewers bool
	Draft          bool
}

// ValidatePROptions validates that the provided PR options are supported by the given capabilities.
func ValidatePROptions(caps *Capabilities, opts *PROptions) error {
	if opts == nil {
		return nil
	}

	if caps == nil {
		caps = &Capabilities{}
	}

	if !caps.TeamReviewers && len(opts.TeamReviewers) > 0 {
		return fmt.Errorf("provider does not support team reviewers")
	}

	if !caps.ResetReviewers && opts.ResetReviewers {
		return fmt.Errorf("provider does not support resetting reviewers")
	}

	if !caps.Draft && opts.Draft != nil {
		return fmt.Errorf("provider does not support draft pull requests")
	}

	return nil
}
