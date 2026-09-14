package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestNoShorthandConflicts walks the entire command tree and verifies that no command
// defines a shorthand that is already claimed by an inherited persistent flag. Cobra
// merges persistent flags from every ancestor into a command's flag set before parsing,
// and pflag panics on a shorthand collision, so such a conflict makes the command
// unusable at runtime rather than failing at registration time.
func TestNoShorthandConflicts(t *testing.T) {
	loadFixture(t)

	walkCommands(t, RootCmd(), nil)
}

// walkCommands recursively checks cmd against the persistent flags of its ancestors.
func walkCommands(t *testing.T, cmd *cobra.Command, inherited []*pflag.Flag) {
	t.Helper()

	// Intentionally use Flags()/PersistentFlags() rather than LocalFlags() or
	// InheritedFlags(), because the latter trigger the persistent flag merge that panics.
	own := make(map[string]*pflag.Flag)
	collect := func(flag *pflag.Flag) {
		if flag.Shorthand != "" {
			own[flag.Shorthand] = flag
		}
	}

	cmd.Flags().VisitAll(collect)
	cmd.PersistentFlags().VisitAll(collect)

	for _, parent := range inherited {
		if cmd.Flags().Lookup(parent.Name) != nil || cmd.PersistentFlags().Lookup(parent.Name) != nil {
			// Same name shadows the inherited flag, so it is never merged.
			continue
		}

		if conflict, ok := own[parent.Shorthand]; ok && conflict.Name != parent.Name {
			t.Errorf("%q defines -%s for --%s, which conflicts with inherited --%s",
				cmd.CommandPath(), parent.Shorthand, conflict.Name, parent.Name)
		}
	}

	inherited = append(inherited, persistentFlags(cmd)...)

	for _, sub := range cmd.Commands() {
		walkCommands(t, sub, inherited)
	}
}

// persistentFlags returns the shorthand-bearing persistent flags declared on cmd.
func persistentFlags(cmd *cobra.Command) []*pflag.Flag {
	flags := make([]*pflag.Flag, 0)

	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Shorthand != "" {
			flags = append(flags, flag)
		}
	})

	return flags
}

// TestCommandsParseFlags ensures every command in the tree can parse its own flag set,
// which is the point at which a shorthand conflict would panic in normal use.
func TestCommandsParseFlags(t *testing.T) {
	loadFixture(t)

	var check func(cmd *cobra.Command)
	check = func(cmd *cobra.Command) {
		t.Run(strings.ReplaceAll(cmd.CommandPath(), " ", "_"), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic while merging flags for %q: %v", cmd.CommandPath(), r)
				}
			}()

			cmd.InheritedFlags()
			cmd.LocalFlags()
		})

		for _, sub := range cmd.Commands() {
			check(sub)
		}
	}

	check(RootCmd())
}
