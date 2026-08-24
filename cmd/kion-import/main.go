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

	"github.com/spf13/cobra"

	"terraform-provider-kion/codegen"
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

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
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

	ctx := context.Background()
	client := kimport.NewClient(flagURL, flagAPIKey, flagSkipSSL)

	fmt.Printf("Enumerating %s (%d resource types)\n", flagURL, len(manifest.Resources))
	results := make([]kimport.Result, 0, len(manifest.Resources))
	for _, r := range manifest.Resources {
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
