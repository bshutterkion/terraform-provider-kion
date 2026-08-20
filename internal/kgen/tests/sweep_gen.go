package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"terraform-provider-kion/internal/conns"
)

func generateSweepFile(projectRoot, pkgName, typeName string, force bool) error {
	outFile := filepath.Join(projectRoot, "internal", "service", pkgName, "sweep.go")

	snake := snakeNameFromTypeName(typeName)
	pascal := pascalName(snake)

	content := buildSweepFile(pkgName, typeName, pascal)

	_, err := writeFileIfNeeded(outFile, content, force)
	return err
}

func buildSweepFile(pkgName, typeName, pascal string) string {
	meta := GetMeta(typeName)
	hasSDK := meta != nil && meta.SDKDeleteMethod != ""

	var b strings.Builder

	fmt.Fprintf(&b, "package %s\n\n", pkgName)
	b.WriteString("import (\n")

	if hasSDK {
		b.WriteString("\t\"fmt\"\n\n")
		b.WriteString("\t\"terraform-provider-kion/internal/conns\"\n\n")
	}

	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/helper/resource\"\n")
	b.WriteString(")\n\n")
	b.WriteString("func init() {\n")
	fmt.Fprintf(&b, "\tresource.AddTestSweepers(%q, &resource.Sweeper{\n", typeName)
	fmt.Fprintf(&b, "\t\tName: %q,\n", typeName)
	fmt.Fprintf(&b, "\t\tF:    sweep%s,\n", pascal)
	b.WriteString("\t})\n")
	b.WriteString("}\n\n")

	if hasSDK {
		fmt.Fprintf(&b, "func sweep%s(_ string) error {\n", pascal)
		b.WriteString("\tconn, err := conns.SharedClient()\n")
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\treturn fmt.Errorf(\"getting shared client: %w\", err)\n")
		b.WriteString("\t}\n")
		b.WriteString("\t_ = conn\n\n")
		fmt.Fprintf(&b, "\t// TODO: List all %s resources with \"test-acc\" prefix and delete them.\n", typeName)
		b.WriteString("\t// Use conn.Client to call the appropriate list endpoint,\n")
		fmt.Fprintf(&b, "\t// then call conn.Client.%s() for each matching resource.\n", meta.SDKDeleteMethod)
		b.WriteString("\treturn nil\n")
		b.WriteString("}\n")
	} else {
		fmt.Fprintf(&b, "func sweep%s(_ string) error {\n", pascal)
		fmt.Fprintf(&b, "\t// TODO: List all %s resources.\n", typeName)
		b.WriteString("\t// Delete any with \"test-acc\" prefix in their name.\n")
		b.WriteString("\treturn nil\n")
		b.WriteString("}\n")
	}

	return b.String()
}

func generateSweepEntrypoint(projectRoot string, pkgs []conns.ServicePackage, force bool) error {
	sweepDir := filepath.Join(projectRoot, "internal", "sweep")
	outFile := filepath.Join(sweepDir, "sweep_test.go")

	ctx := context.Background()

	// Collect all package names that have resources (and thus sweepers).
	var pkgNames []string
	for _, pkg := range pkgs {
		if len(pkg.Resources(ctx)) > 0 {
			pkgNames = append(pkgNames, pkg.ServicePackageName())
		}
	}

	content := buildSweepEntrypoint(pkgNames)

	_, err := writeFileIfNeeded(outFile, content, force)
	return err
}

func buildSweepEntrypoint(pkgNames []string) string {
	var b strings.Builder

	b.WriteString("package sweep_test\n\n")
	b.WriteString("import (\n")
	b.WriteString("\t\"testing\"\n\n")
	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/helper/resource\"\n\n")
	b.WriteString("\t// Import all service packages to register their sweepers.\n")
	for _, name := range pkgNames {
		fmt.Fprintf(&b, "\t_ \"terraform-provider-kion/internal/service/%s\"\n", name)
	}
	b.WriteString(")\n\n")
	b.WriteString("func TestMain(m *testing.M) {\n")
	b.WriteString("\tresource.TestMain(m)\n")
	b.WriteString("}\n")

	return b.String()
}
