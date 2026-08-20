//go:build kgendocs

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExamplesCommand_Metadata asserts wiring for the examples subcommand
// without running RunE (which generates .tf files).
func TestExamplesCommand_Metadata(t *testing.T) {
	assert.Equal(t, "examples", examplesCmd.Use)
	assert.NotEmpty(t, examplesCmd.Short)
	assert.NotNil(t, examplesCmd.RunE)

	resourceFlag := examplesCmd.Flags().Lookup("resource")
	require.NotNil(t, resourceFlag, "examples should expose --resource")
	assert.Empty(t, resourceFlag.Shorthand, "--resource has no shorthand")

	forceFlag := examplesCmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "examples should expose --force")
	assert.Equal(t, "f", forceFlag.Shorthand)

	assert.Same(t, examplesCmd, findSubcommand(rootCmd, "examples"))
}

// TestTestsCommand_Metadata asserts wiring for the tests subcommand without
// running RunE (which generates acceptance test files).
func TestTestsCommand_Metadata(t *testing.T) {
	assert.Equal(t, "tests", testsCmd.Use)
	assert.NotEmpty(t, testsCmd.Short)
	assert.NotEmpty(t, testsCmd.Long, "tests command documents usage in Long")
	assert.NotNil(t, testsCmd.RunE)

	resourceFlag := testsCmd.Flags().Lookup("resource")
	require.NotNil(t, resourceFlag, "tests should expose --resource")

	sweepFlag := testsCmd.Flags().Lookup("sweep-only")
	require.NotNil(t, sweepFlag, "tests should expose --sweep-only")

	forceFlag := testsCmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "tests should expose --force")
	assert.Equal(t, "f", forceFlag.Shorthand)

	assert.Same(t, testsCmd, findSubcommand(rootCmd, "tests"))
}

// TestAllSubcommands_HaveShortDescriptions is a blanket check that every
// registered subcommand carries a non-empty Short (help output hygiene).
func TestAllSubcommands_HaveShortDescriptions(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		// Skip cobra's auto-injected help/completion commands.
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		assert.NotEmptyf(t, c.Short, "subcommand %q should have a Short description", c.Name())
	}
}

// TestForceFlag_DefaultsToFalse verifies the destructive --force flag defaults
// off across every command that exposes it.
func TestForceFlag_DefaultsToFalse(t *testing.T) {
	cmds := []*cobra.Command{resourceCmd, datasourceCmd, serviceCmd, examplesCmd, testsCmd}
	for _, c := range cmds {
		f := c.Flags().Lookup("force")
		require.NotNilf(t, f, "%s should have --force", c.Name())
		assert.Equalf(t, "false", f.DefValue, "%s --force should default to false", c.Name())
	}
}
