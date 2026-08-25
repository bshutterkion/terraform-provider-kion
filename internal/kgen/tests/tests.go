// Package tests generates acceptance test files from registered resource and data source schemas.
package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"

	"terraform-provider-kion/internal/provider"
)

const providerTypeName = "kion"

// Generate generates acceptance test files for all registered resources and data sources.
// If filterResource is non-empty, only that resource/data source is generated.
// Sweepers are NOT written here: kgen crud resolves each resource's real list
// and delete ops and emits internal/service/<n>/sweep.go. This generator used to
// emit a registering stub that returned nil, which reported success to
// `make sweep` while orphaned test-acc records piled up.
func Generate(force bool, filterResource string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pkgs := provider.ServicePackages()

	var generated int
	for _, pkg := range pkgs {
		pkgName := pkg.ServicePackageName()

		for _, r := range pkg.Resources(ctx) {
			res := r.Factory()
			typeName := resourceTypeName(ctx, res)
			if filterResource != "" && typeName != filterResource {
				continue
			}

			if err := generateResourceTest(projectRoot, pkgName, typeName, res, force); err != nil {
				return fmt.Errorf("generating resource test for %s: %w", typeName, err)
			}
			generated++
		}

		for _, d := range pkg.DataSources(ctx) {
			ds := d.Factory()
			typeName := dataSourceTypeName(ctx, ds)
			if filterResource != "" && typeName != filterResource {
				continue
			}

			// Find matching resource for the data source config
			var matchingResource fwresource.Resource
			var matchingResTypeName string
			for _, r := range pkg.Resources(ctx) {
				res := r.Factory()
				matchingResource = res
				matchingResTypeName = resourceTypeName(ctx, res)
				break
			}

			if err := generateDataSourceTest(projectRoot, pkgName, typeName, ds, matchingResource, matchingResTypeName, force); err != nil {
				return fmt.Errorf("generating data source test for %s: %w", typeName, err)
			}
			generated++
		}
	}

	if filterResource != "" && generated == 0 {
		return fmt.Errorf("no resource or data source found matching %q", filterResource)
	}

	fmt.Printf("Generated %d test file(s)\n", generated)
	return nil
}

// resourceTypeName calls Metadata() on a resource to get its full type name.
func resourceTypeName(ctx context.Context, r fwresource.Resource) string {
	req := fwresource.MetadataRequest{ProviderTypeName: providerTypeName}
	resp := &fwresource.MetadataResponse{}
	r.Metadata(ctx, req, resp)
	return resp.TypeName
}

// dataSourceTypeName calls Metadata() on a data source to get its full type name.
func dataSourceTypeName(ctx context.Context, d datasource.DataSource) string {
	req := datasource.MetadataRequest{ProviderTypeName: providerTypeName}
	resp := &datasource.MetadataResponse{}
	d.Metadata(ctx, req, resp)
	return resp.TypeName
}

// snakeNameFromTypeName strips the "kion_" prefix to get the snake_case name.
func snakeNameFromTypeName(typeName string) string {
	return strings.TrimPrefix(typeName, "kion_")
}

// pascalName converts a snake_case name to PascalCase.
func pascalName(snake string) string {
	parts := strings.Split(snake, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	for {
		if _, err := fsw.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func writeFileIfNeeded(outFile string, content string, force bool) (bool, error) {
	if !force {
		if _, err := fsw.Stat(outFile); err == nil {
			fmt.Printf("  skip %s (exists, use --force to overwrite)\n", outFile)
			return false, nil
		}
	}

	dir := filepath.Dir(outFile)
	if err := fsw.MkdirAll(dir, 0750); err != nil {
		return false, fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := fsw.WriteFile(outFile, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("writing %s: %w", outFile, err)
	}
	fmt.Printf("  wrote %s\n", outFile)
	return true, nil
}
