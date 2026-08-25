package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"terraform-provider-kion/codegen"
	"terraform-provider-kion/internal/kgen/references"
	"terraform-provider-kion/internal/kimport/rewriteref"
)

var (
	flagRefImports   string
	flagRefGenerated string
	flagRefOut       string
	flagRefDryRun    bool
)

func newRewriteRefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rewrite-refs",
		Short: "Turn literal foreign keys in a generated configuration into references",
		Long: `Rewrites the literal ids that terraform plan -generate-config-out writes
into references between resource blocks:

  ou_id = 11   ->   ou_id = kion_ou.kion_ou_11.id

A generated configuration is correct for the install it came from and does not
express its own dependency graph, so destroy-and-reapply will not order
correctly and recreating a parent will not cascade. Applied to a DIFFERENT
install the literal either dangles or silently binds to whatever record happens
to hold that id there. References remove both problems: Terraform resolves them
from the ids it creates, which is why cloning an install needs no id map.

Both files are required. generated.tf holds no ids -- they are Computed, so
-generate-config-out omits them -- so the id to label index comes from the
import blocks, where "to" and "id" sit together.

  kion-import --url https://kion.example.com --out imports.tf
  terraform plan -generate-config-out=generated.tf
  kion-import rewrite-refs --imports imports.tf --generated generated.tf

Ids whose referent is not managed in this configuration are LEFT AS LITERALS and
reported. Those are the boundary: users, IDMS, stock permission schemes. Resolve
them with a data source, a variable, or by importing them too.`,
		RunE: runRewriteRefs,
	}
	cmd.Flags().StringVar(&flagRefImports, "imports", "imports.tf",
		"the import blocks written by kion-import (supplies the id -> label index)")
	cmd.Flags().StringVar(&flagRefGenerated, "generated", "generated.tf",
		"the configuration written by terraform plan -generate-config-out")
	cmd.Flags().StringVar(&flagRefOut, "out", "",
		"write here instead of rewriting --generated in place")
	cmd.Flags().BoolVar(&flagRefDryRun, "dry-run", false,
		"report what would change and write nothing")
	return cmd
}

func runRewriteRefs(cmd *cobra.Command, _ []string) error {
	refs, err := references.Parse(codegen.ReferencesYAML)
	if err != nil {
		return err
	}
	importsRaw, err := os.ReadFile(flagRefImports)
	if err != nil {
		return fmt.Errorf("reading imports: %w", err)
	}
	generatedRaw, err := os.ReadFile(flagRefGenerated)
	if err != nil {
		return fmt.Errorf("reading generated config: %w", err)
	}

	idx, err := rewriteref.BuildIndex(importsRaw, flagRefImports)
	if err != nil {
		return err
	}
	out, res, err := rewriteref.Rewrite(generatedRaw, flagRefGenerated, idx, refs)
	if err != nil {
		return err
	}

	report(cmd.OutOrStdout(), res)

	if flagRefDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\nDry run. Nothing written.")
		return nil
	}
	dst := flagRefOut
	if dst == "" {
		dst = flagRefGenerated
	}
	if err := os.WriteFile(dst, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nWrote %s\n", dst)
	return nil
}

// report prints what was rewritten and, more importantly, what was not. An
// unresolved reference is not an error -- it is the edge of what this
// configuration manages -- but it must never be silent, or the operator ends up
// with literals they believe are references.
func report(w io.Writer, res rewriteref.Result) {
	fmt.Fprintf(w, "%d reference(s) rewritten across %d resource block(s)\n", res.Rewritten, res.Blocks)
	if len(res.Unresolved) == 0 {
		return
	}

	counts := res.TargetCounts()
	targets := make([]string, 0, len(counts))
	for t := range counts {
		targets = append(targets, t)
	}
	sort.Strings(targets)

	fmt.Fprintf(w, "\n%d reference(s) left as literals -- their referent is not managed here:\n", len(res.Unresolved))
	for _, t := range targets {
		fmt.Fprintf(w, "  %-40s %d\n", t, counts[t])
	}
	fmt.Fprintln(w, "\nEach is a boundary of this configuration. Resolve by importing that")
	fmt.Fprintln(w, "resource too, or by replacing the literal with a data source or a variable:")
	fmt.Fprintf(w, "\n  data \"%s\" \"example\" { filter { ... } }\n", targets[0])

	// The detail is what makes this actionable, but a full list of 10k lines
	// is not. Show enough to act on and say how many were withheld.
	const show = 20
	fmt.Fprintln(w, "\nFirst few:")
	for i, u := range res.Unresolved {
		if i == show {
			fmt.Fprintf(w, "  ... and %d more\n", len(res.Unresolved)-show)
			break
		}
		fmt.Fprintf(w, "  %s.%s = %s\n", u.Resource, u.Attr, u.Value)
	}
}
