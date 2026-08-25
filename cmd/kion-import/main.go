// Command kion-import enumerates a live Kion install into Terraform import
// blocks.
//
// It deliberately does NOT generate configuration or state. Every resource the
// provider serves implements ImportState, so `terraform plan
// -generate-config-out` produces the configuration and `terraform apply` writes
// the state -- both through the provider's own Read, which is correct by
// construction. What Terraform cannot do is discover what exists in Kion. That
// is this tool's only job.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"terraform-provider-kion/codegen"
	"terraform-provider-kion/internal/kgen/importmanifest"
	"terraform-provider-kion/internal/kimport"
)

var (
	flagURL             string
	flagAPIKey          string
	flagOut             string
	flagProviderVersion string
	flagSkipSSL         bool
	flagManifest        string
	flagProbe           bool
	flagAPIPrefix       string
	flagInclude         []string
	flagExclude         []string
	flagSelection       string
	flagListTypes       bool
)

func main() {
	root := &cobra.Command{
		Use:   "kion-import",
		Short: "Enumerate a Kion install into Terraform import blocks",
		Long: `Reads every resource the Kion provider supports from a live install and
writes a Terraform configuration of import blocks.

  kion-import --url https://kion.example.com --out imports.tf
  terraform init
  terraform plan -generate-config-out=generated.tf
  terraform apply

The API key is read from --api-key or the KION_APIKEY environment variable.`,
		RunE: run,
	}

	root.Flags().StringVar(&flagURL, "url", os.Getenv("KION_URL"), "Kion install URL")
	root.Flags().StringVar(&flagAPIKey, "api-key", os.Getenv("KION_APIKEY"), "Kion app API key")
	root.Flags().StringVar(&flagOut, "out", "imports.tf", "output file")
	root.Flags().StringVar(&flagProviderVersion, "provider-version", "1.0.0",
		"provider version constraint written to the generated config")
	root.Flags().BoolVar(&flagSkipSSL, "skip-ssl-validation", false, "skip TLS verification")
	root.Flags().StringVar(&flagManifest, "manifest", "",
		"override the embedded import manifest (maintainers)")
	root.Flags().BoolVar(&flagProbe, "probe", false,
		"report per-resource read outcomes and write nothing")
	root.Flags().StringSliceVar(&flagInclude, "include", nil,
		"only these resource types (repeatable, comma-separated); default is everything readable")
	root.Flags().StringSliceVar(&flagExclude, "exclude", nil,
		"skip these resource types (repeatable, comma-separated), applied after --include")
	root.Flags().StringVar(&flagSelection, "selection", "",
		"YAML file with include/exclude lists; --include/--exclude add to it")
	root.Flags().BoolVar(&flagListTypes, "list-types", false,
		"print the resource types this build can import and exit")
	root.Flags().StringVar(&flagAPIPrefix, "api-prefix", "/api",
		"path prefix the install serves its API under, appended to --url. "+
			"Hosted installs serve under /api (the default); set to \"\" only when hitting an app "+
			"directly (e.g. on localhost), where the API is served at the root")

	root.AddCommand(newRewriteRefsCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	// --list-types answers "what can I put in --include" and reads only the
	// embedded manifest, so it must not demand credentials.
	if flagListTypes {
		manifest, err := kimport.LoadManifest(codegen.ImportManifestJSON)
		if err != nil {
			return err
		}
		return listTypes(manifest.Resources)
	}
	if flagURL == "" {
		return fmt.Errorf("--url is required (or set KION_URL)")
	}
	if flagAPIKey == "" {
		return fmt.Errorf("--api-key is required (or set KION_APIKEY)")
	}

	data := codegen.ImportManifestJSON
	if flagManifest != "" {
		var err error
		if data, err = os.ReadFile(flagManifest); err != nil {
			return err
		}
	}
	manifest, err := kimport.LoadManifest(data)
	if err != nil {
		return err
	}

	if flagListTypes {
		return listTypes(manifest.Resources)
	}

	sel, err := kimport.LoadSelection(flagSelection)
	if err != nil {
		return err
	}
	selected, err := sel.Merge(flagInclude, flagExclude).Apply(manifest.Resources)
	if err != nil {
		return err
	}

	ctx := context.Background()
	client := kimport.NewClient(flagURL, flagAPIKey, flagSkipSSL, flagAPIPrefix)

	scope := fmt.Sprintf("%d resource types", len(selected))
	if len(selected) != len(manifest.Resources) {
		scope = fmt.Sprintf("%d of %d resource types", len(selected), len(manifest.Resources))
	}
	fmt.Printf("Enumerating %s (%s)\n", flagURL, scope)
	results := make([]kimport.Result, 0, len(selected))
	for _, r := range selected {
		results = append(results, kimport.Enumerate(ctx, client, r))
	}

	if flagProbe {
		for _, r := range results {
			fmt.Printf("%-45s %-12s %5d  %s\n", r.TFType, r.Status, len(r.Records), r.Reason)
		}
		fmt.Print(kimport.FormatReport(results))
		return nil
	}

	out := kimport.RenderImports(results, flagProviderVersion)
	if err := os.WriteFile(flagOut, []byte(out), 0o644); err != nil { //nolint:gosec // user-specified output
		return err
	}
	fmt.Printf("Wrote %s\n", flagOut)
	fmt.Print(kimport.FormatReport(results))
	fmt.Printf("\nNext:\n  terraform init\n  terraform plan -generate-config-out=generated.tf\n")
	return nil
}

// listTypes prints what this build can import, so --include and --exclude can be
// written without opening the manifest. Aliases are shown against the type they
// duplicate rather than hidden, because a configuration written for the old
// provider names them and an operator needs to know what to use instead.
func listTypes(rs []importmanifest.Resource) error {
	sort.Slice(rs, func(i, j int) bool { return rs[i].TFType < rs[j].TFType })
	var readable, skipped int
	for _, r := range rs {
		switch {
		case r.AliasOf != "":
			fmt.Printf("  %-46s alias of %s\n", r.TFType, r.AliasOf)
			skipped++
		case !r.Readable:
			fmt.Printf("  %-46s not importable: %s\n", r.TFType, r.Reason)
			skipped++
		default:
			fmt.Printf("  %s\n", r.TFType)
			readable++
		}
	}
	fmt.Printf("\n%d importable, %d not\n", readable, skipped)
	return nil
}
