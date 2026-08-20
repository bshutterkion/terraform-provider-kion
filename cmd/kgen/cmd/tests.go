//go:build kgendocs

// The tests command imports internal/provider (→ every service package), so it is
// behind the `kgendocs` build tag — see examples.go. Build with `-tags kgendocs`.
package cmd

import (
	"terraform-provider-kion/internal/kgen/tests"

	"github.com/spf13/cobra"
)

var (
	testsFilterResource string
	testsSweepOnly      bool
	testsForce          bool
)

var testsCmd = &cobra.Command{
	Use:   "tests",
	Short: "Generate acceptance test files from resource/data source schemas",
	Long: `Generate acceptance test files for all registered resources and data sources.

By default, existing test files are skipped. Use --force to overwrite.

Examples:
  kgen tests                           # Generate all missing test files
  kgen tests --resource kion_label     # Generate for one resource only
  kgen tests --force                   # Overwrite existing test files
  kgen tests --sweep-only              # Only generate sweep files`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return tests.Generate(testsForce, testsFilterResource, testsSweepOnly)
	},
}

func init() {
	testsCmd.Flags().StringVar(&testsFilterResource, "resource", "", "generate only for a specific resource (e.g., kion_label)")
	testsCmd.Flags().BoolVar(&testsSweepOnly, "sweep-only", false, "only generate sweep files")
	testsCmd.Flags().BoolVarP(&testsForce, "force", "f", false, "force creation, overwriting existing files")
	rootCmd.AddCommand(testsCmd)
}
