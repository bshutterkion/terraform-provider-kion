package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func generateDataSourceTest(projectRoot, pkgName, typeName string, ds datasource.DataSource, matchingResource resource.Resource, matchingResourceTypeName string, force bool) error {
	outFile := filepath.Join(projectRoot, "internal", "service", pkgName, pkgName+"_data_source_test.go")

	snake := snakeNameFromTypeName(typeName)
	pascal := pascalName(snake)

	schemaReq := datasource.SchemaRequest{}
	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), schemaReq, schemaResp)

	var resSchema *rsschema.Schema
	if matchingResource != nil {
		req := resource.SchemaRequest{}
		resp := &resource.SchemaResponse{}
		matchingResource.Schema(context.Background(), req, resp)
		resSchema = &resp.Schema
	}

	content := buildDataSourceTestFile(pkgName, typeName, snake, pascal, schemaResp.Schema, matchingResourceTypeName, resSchema)

	_, err := writeFileIfNeeded(outFile, content, force)
	return err
}

func buildDataSourceTestFile(pkgName, typeName, _, pascal string, s dsschema.Schema, resourceTypeName string, resSchema *rsschema.Schema) string {
	var b strings.Builder

	hasResourceName := resourceTypeName != "" && resSchema != nil && hasNameAttrInResourceSchema(resSchema)
	resMeta := GetMeta(resourceTypeName)
	needsFmt := hasResourceName || metaHasFormatVerbs(resMeta)

	fmt.Fprintf(&b, "package %s_test\n\n", pkgName)
	b.WriteString("import (\n")
	if needsFmt {
		b.WriteString("\t\"fmt\"\n")
	}
	b.WriteString("\t\"testing\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/helper/resource\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"terraform-provider-kion/internal/acctest\"\n")
	b.WriteString(")\n\n")

	// TestAccKion<Name>DataSource_basic
	fmt.Fprintf(&b, "func TestAccKion%sDataSource_basic(t *testing.T) {\n", pascal)
	b.WriteString("\tif testing.Short() {\n")
	b.WriteString("\t\tt.Skip(\"skipping long-running test in short mode\")\n")
	b.WriteString("\t}\n\n")
	if needsFmt {
		b.WriteString("\trName := acctest.RandomWithPrefix(acctest.ResourcePrefix)\n")
	}
	fmt.Fprintf(&b, "\tdataSourceName := %q\n\n", "data."+typeName+".test")
	b.WriteString("\tresource.Test(t, resource.TestCase{\n")
	b.WriteString("\t\tPreCheck:                 func() { acctest.PreCheck(t) },\n")
	b.WriteString("\t\tProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,\n")
	b.WriteString("\t\tSteps: []resource.TestStep{\n")
	b.WriteString("\t\t\t{\n")
	if needsFmt {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sDataSourceConfig_basic(rName),\n", pascal)
	} else {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sDataSourceConfig_basic(),\n", pascal)
	}
	b.WriteString("\t\t\t\tCheck: resource.ComposeAggregateTestCheckFunc(\n")
	b.WriteString("\t\t\t\t\tresource.TestCheckResourceAttrSet(dataSourceName, \"id\"),\n")

	// Add checks for computed data source attributes
	dsAttrNames := sortedDSKeys(s.Attributes)
	for _, name := range dsAttrNames {
		if name == "id" {
			continue
		}
		attr := s.Attributes[name]
		if !isDSComputedOnly(attr) {
			continue
		}
		fmt.Fprintf(&b, "\t\t\t\t\tresource.TestCheckResourceAttrSet(dataSourceName, %q),\n", name)
	}

	b.WriteString("\t\t\t\t),\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t})\n")
	b.WriteString("}\n\n")

	// testAcc<Name>DataSourceConfig_basic
	if resourceTypeName != "" && resSchema != nil {
		requiredAttrs := getRequiredResourceAttrs(*resSchema)
		hasName := hasNameField(requiredAttrs)
		meta := GetMeta(resourceTypeName)
		dsNeedsRName := hasName || metaHasFormatVerbs(meta)

		if dsNeedsRName {
			fmt.Fprintf(&b, "func testAcc%sDataSourceConfig_basic(rName string) string {\n", pascal)
			b.WriteString("\treturn fmt.Sprintf(`\n")
		} else {
			fmt.Fprintf(&b, "func testAcc%sDataSourceConfig_basic() string {\n", pascal)
			b.WriteString("\treturn `\n")
		}

		// Dependency resources from the registry
		if meta != nil {
			for _, dep := range meta.Dependencies {
				fmt.Fprintf(&b, "resource %q %q {\n", dep.TypeName, dep.RefName)
				depFieldNames := sortedMapKeys(dep.Fields)
				for _, fn := range depFieldNames {
					fmt.Fprintf(&b, "  %s = %s\n", fn, dep.Fields[fn])
				}
				b.WriteString("}\n\n")
			}
		}

		// Resource prerequisite
		fmt.Fprintf(&b, "resource %q %q {\n", resourceTypeName, "test")
		for _, attr := range requiredAttrs {
			val := basicTestValueWithMeta(attr, dsNeedsRName, meta)
			fmt.Fprintf(&b, "  %s = %s\n", attr.name, val)
		}

		// Dependency references and extra blocks
		if meta != nil {
			for _, dep := range meta.Dependencies {
				if !attrInList(dep.TargetField, requiredAttrs) {
					fmt.Fprintf(&b, "  %s = %s.%s.%s\n", dep.TargetField, dep.TypeName, dep.RefName, dep.RefAttribute)
				}
			}
			for _, block := range meta.ExtraHCLBlocks {
				fmt.Fprintf(&b, "  %s\n", block)
			}
		}

		b.WriteString("}\n\n")

		// Data source reading it back
		fmt.Fprintf(&b, "data %q %q {\n", typeName, "test")
		fmt.Fprintf(&b, "  id = %s.test.id\n", resourceTypeName)
		b.WriteString("}\n")

		if dsNeedsRName {
			b.WriteString("`, rName)\n")
		} else {
			b.WriteString("`\n")
		}
	} else {
		// No matching resource, generate standalone data source config
		fmt.Fprintf(&b, "func testAcc%sDataSourceConfig_basic() string {\n", pascal)
		b.WriteString("\treturn `\n")
		fmt.Fprintf(&b, "data %q %q {\n", typeName, "test")
		b.WriteString("  # TODO: Fill in filter criteria or ID to look up the data source.\n")
		b.WriteString("}\n")
		b.WriteString("`\n")
	}
	b.WriteString("}\n")

	return b.String()
}

func hasNameAttrInResourceSchema(s *rsschema.Schema) bool {
	for name, attr := range s.Attributes {
		if name == "name" && isResourceRequired(attr) {
			return true
		}
	}
	return false
}

func sortedDSKeys(m map[string]dsschema.Attribute) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isDSComputedOnly(attr dsschema.Attribute) bool {
	switch a := attr.(type) {
	case dsschema.StringAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.Int64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.BoolAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.Float64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.MapAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.ListAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.SetAttribute:
		return a.Computed && !a.Required && !a.Optional
	default:
		return false
	}
}
