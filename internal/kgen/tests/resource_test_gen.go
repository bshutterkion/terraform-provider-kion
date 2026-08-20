package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func generateResourceTest(projectRoot, pkgName, typeName string, res resource.Resource, force bool) error {
	outFile := filepath.Join(projectRoot, "internal", "service", pkgName, pkgName+"_test.go")

	snake := snakeNameFromTypeName(typeName)
	pascal := pascalName(snake)

	schemaReq := resource.SchemaRequest{}
	schemaResp := &resource.SchemaResponse{}
	res.Schema(context.Background(), schemaReq, schemaResp)

	content := buildResourceTestFile(pkgName, typeName, snake, pascal, schemaResp.Schema)

	_, err := writeFileIfNeeded(outFile, content, force)
	return err
}

func buildResourceTestFile(pkgName, typeName, snake, pascal string, s rsschema.Schema) string {
	requiredAttrs := getRequiredResourceAttrs(s)
	hasName := hasNameField(requiredAttrs)
	meta := GetMeta(typeName)
	hasSDK := meta != nil && meta.SDKGetMethod != ""
	needsRName := hasName || metaHasFormatVerbs(meta)

	var b strings.Builder

	fmt.Fprintf(&b, "package %s_test\n\n", pkgName)
	b.WriteString("import (\n")
	b.WriteString("\t\"context\"\n")
	b.WriteString("\t\"fmt\"\n")
	if hasSDK {
		b.WriteString("\t\"strconv\"\n")
	}
	b.WriteString("\t\"testing\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/helper/resource\"\n")
	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/terraform\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"terraform-provider-kion/internal/acctest\"\n")
	if hasSDK {
		b.WriteString("\t\"terraform-provider-kion/internal/errs\"\n")
		b.WriteString("\n")
		// SDKImportPath is defined in config.go and pinned per release branch.
		b.WriteString("\tgenerated \"" + SDKImportPath + "\"\n")
	}
	b.WriteString(")\n\n")

	// TestAccKion<Name>_basic
	fmt.Fprintf(&b, "func TestAccKion%s_basic(t *testing.T) {\n", pascal)
	b.WriteString("\tif testing.Short() {\n")
	b.WriteString("\t\tt.Skip(\"skipping long-running test in short mode\")\n")
	b.WriteString("\t}\n\n")
	b.WriteString("\tctx := acctest.Context(t)\n")
	if needsRName {
		b.WriteString("\trName := acctest.RandomWithPrefix(acctest.ResourcePrefix)\n")
	}
	fmt.Fprintf(&b, "\tresourceName := %q\n\n", typeName+".test")
	b.WriteString("\tresource.Test(t, resource.TestCase{\n")
	b.WriteString("\t\tPreCheck:                 func() { acctest.PreCheck(t) },\n")
	b.WriteString("\t\tProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,\n")
	fmt.Fprintf(&b, "\t\tCheckDestroy:             testAccCheck%sDestroy(ctx),\n", pascal)
	b.WriteString("\t\tSteps: []resource.TestStep{\n")
	b.WriteString("\t\t\t{\n")
	if needsRName {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_basic(rName),\n", pascal)
	} else {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_basic(),\n", pascal)
	}
	b.WriteString("\t\t\t\tCheck: resource.ComposeAggregateTestCheckFunc(\n")
	fmt.Fprintf(&b, "\t\t\t\t\ttestAccCheck%sExists(ctx, resourceName),\n", pascal)
	b.WriteString("\t\t\t\t\tresource.TestCheckResourceAttrSet(resourceName, \"id\"),\n")
	// Add checks for required attrs
	for _, attr := range requiredAttrs {
		if attr.name == "name" {
			fmt.Fprintf(&b, "\t\t\t\t\tresource.TestCheckResourceAttr(resourceName, %q, rName),\n", attr.name)
		} else {
			fmt.Fprintf(&b, "\t\t\t\t\tresource.TestCheckResourceAttrSet(resourceName, %q),\n", attr.name)
		}
	}
	b.WriteString("\t\t\t\t),\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\t{\n")
	b.WriteString("\t\t\t\tResourceName:      resourceName,\n")
	b.WriteString("\t\t\t\tImportState:       true,\n")
	b.WriteString("\t\t\t\tImportStateVerify: true,\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t})\n")
	b.WriteString("}\n\n")

	// TestAccKion<Name>_update
	fmt.Fprintf(&b, "func TestAccKion%s_update(t *testing.T) {\n", pascal)
	b.WriteString("\tif testing.Short() {\n")
	b.WriteString("\t\tt.Skip(\"skipping long-running test in short mode\")\n")
	b.WriteString("\t}\n\n")
	b.WriteString("\tctx := acctest.Context(t)\n")
	if needsRName {
		b.WriteString("\trName := acctest.RandomWithPrefix(acctest.ResourcePrefix)\n")
	}
	fmt.Fprintf(&b, "\tresourceName := %q\n\n", typeName+".test")
	b.WriteString("\tresource.Test(t, resource.TestCase{\n")
	b.WriteString("\t\tPreCheck:                 func() { acctest.PreCheck(t) },\n")
	b.WriteString("\t\tProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,\n")
	fmt.Fprintf(&b, "\t\tCheckDestroy:             testAccCheck%sDestroy(ctx),\n", pascal)
	b.WriteString("\t\tSteps: []resource.TestStep{\n")
	b.WriteString("\t\t\t{\n")
	if needsRName {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_basic(rName),\n", pascal)
	} else {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_basic(),\n", pascal)
	}
	b.WriteString("\t\t\t\tCheck: resource.ComposeAggregateTestCheckFunc(\n")
	fmt.Fprintf(&b, "\t\t\t\t\ttestAccCheck%sExists(ctx, resourceName),\n", pascal)
	b.WriteString("\t\t\t\t\tresource.TestCheckResourceAttrSet(resourceName, \"id\"),\n")
	b.WriteString("\t\t\t\t),\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\t{\n")
	if needsRName {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_update(rName),\n", pascal)
	} else {
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_update(),\n", pascal)
	}
	b.WriteString("\t\t\t\tCheck: resource.ComposeAggregateTestCheckFunc(\n")
	fmt.Fprintf(&b, "\t\t\t\t\ttestAccCheck%sExists(ctx, resourceName),\n", pascal)
	b.WriteString("\t\t\t\t\tresource.TestCheckResourceAttrSet(resourceName, \"id\"),\n")
	b.WriteString("\t\t\t\t),\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\t{\n")
	b.WriteString("\t\t\t\tResourceName:      resourceName,\n")
	b.WriteString("\t\t\t\tImportState:       true,\n")
	b.WriteString("\t\t\t\tImportStateVerify: true,\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t})\n")
	b.WriteString("}\n\n")

	// testAccCheck<Name>Exists
	buildExistsFunc(&b, pascal, typeName, meta)

	// testAccCheck<Name>Destroy
	buildDestroyFunc(&b, pascal, typeName, meta)

	// testAcc<Name>Config_basic
	basicConfig := buildBasicConfig(typeName, requiredAttrs, needsRName, meta)
	if needsRName {
		fmt.Fprintf(&b, "func testAcc%sConfig_basic(rName string) string {\n", pascal)
		b.WriteString("\treturn fmt.Sprintf(`\n")
		b.WriteString(basicConfig)
		b.WriteString("`, rName)\n")
	} else {
		fmt.Fprintf(&b, "func testAcc%sConfig_basic() string {\n", pascal)
		b.WriteString("\treturn `\n")
		b.WriteString(basicConfig)
		b.WriteString("`\n")
	}
	b.WriteString("}\n\n")

	// testAcc<Name>Config_update
	updateConfig := buildUpdateConfig(typeName, requiredAttrs, s, needsRName, meta)
	if needsRName {
		fmt.Fprintf(&b, "func testAcc%sConfig_update(rName string) string {\n", pascal)
		b.WriteString("\treturn fmt.Sprintf(`\n")
		b.WriteString(updateConfig)
		b.WriteString("`, rName)\n")
	} else {
		fmt.Fprintf(&b, "func testAcc%sConfig_update() string {\n", pascal)
		b.WriteString("\treturn `\n")
		b.WriteString(updateConfig)
		b.WriteString("`\n")
	}
	b.WriteString("}\n")

	return b.String()
}

// buildExistsFunc generates the testAccCheck<Name>Exists function.
// When SDK metadata is available, it produces a real API call; otherwise a TODO stub.
func buildExistsFunc(b *strings.Builder, pascal, typeName string, meta *ResourceMeta) {
	if meta != nil && meta.SDKGetMethod != "" {
		fmt.Fprintf(b, "func testAccCheck%sExists(_ context.Context, name string) resource.TestCheckFunc {\n", pascal)
		b.WriteString("\treturn func(s *terraform.State) error {\n")
		b.WriteString("\t\trs, ok := s.RootModule().Resources[name]\n")
		b.WriteString("\t\tif !ok {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"not found: %s\", name)\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\tif rs.Primary.ID == \"\" {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"no ID set for %s\", name)\n")
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\tconn, err := acctest.SharedClient()\n")
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"getting shared client: %w\", err)\n")
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\tid, err := strconv.ParseInt(rs.Primary.ID, 10, 64)\n")
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"parsing ID: %w\", err)\n")
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\tctx := context.Background()\n")
		fmt.Fprintf(b, "\t\tout, err := conn.Client.%s(ctx, %s)\n", meta.SDKGetMethod, meta.SDKGetParams)
		b.WriteString("\t\tif err != nil {\n")
		fmt.Fprintf(b, "\t\t\treturn fmt.Errorf(\"reading %s (%%d): %%w\", id, err)\n", typeName)
		b.WriteString("\t\t}\n")
		b.WriteString("\t\tif errs.IsNotFound(out) {\n")
		fmt.Fprintf(b, "\t\t\treturn fmt.Errorf(\"%s (%%d) not found\", id)\n", typeName)
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	} else {
		// Fallback: TODO stub
		fmt.Fprintf(b, "func testAccCheck%sExists(_ context.Context, name string) resource.TestCheckFunc {\n", pascal)
		b.WriteString("\treturn func(s *terraform.State) error {\n")
		b.WriteString("\t\trs, ok := s.RootModule().Resources[name]\n")
		b.WriteString("\t\tif !ok {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"not found: %s\", name)\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\tif rs.Primary.ID == \"\" {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"no ID set for %s\", name)\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\t// TODO: Call SDK to verify the resource exists.\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	}
}

// buildDestroyFunc generates the testAccCheck<Name>Destroy function.
// When SDK metadata is available, it produces a real API call; otherwise a TODO stub.
func buildDestroyFunc(b *strings.Builder, pascal, typeName string, meta *ResourceMeta) {
	if meta != nil && meta.SDKGetMethod != "" {
		fmt.Fprintf(b, "func testAccCheck%sDestroy(_ context.Context) resource.TestCheckFunc {\n", pascal)
		b.WriteString("\treturn func(s *terraform.State) error {\n")
		b.WriteString("\t\tconn, err := acctest.SharedClient()\n")
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"getting shared client: %w\", err)\n")
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\tfor _, rs := range s.RootModule().Resources {\n")
		fmt.Fprintf(b, "\t\t\tif rs.Type != %q {\n", typeName)
		b.WriteString("\t\t\t\tcontinue\n")
		b.WriteString("\t\t\t}\n\n")
		b.WriteString("\t\t\tid, err := strconv.ParseInt(rs.Primary.ID, 10, 64)\n")
		b.WriteString("\t\t\tif err != nil {\n")
		b.WriteString("\t\t\t\treturn fmt.Errorf(\"parsing ID: %w\", err)\n")
		b.WriteString("\t\t\t}\n\n")
		b.WriteString("\t\t\tctx := context.Background()\n")
		fmt.Fprintf(b, "\t\t\tout, err := conn.Client.%s(ctx, %s)\n", meta.SDKGetMethod, meta.SDKGetParams)
		b.WriteString("\t\t\tif errs.IsNotFound(out) {\n")
		b.WriteString("\t\t\t\tcontinue\n")
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t\tif err != nil {\n")
		fmt.Fprintf(b, "\t\t\t\treturn fmt.Errorf(\"reading %s (%%d): %%w\", id, err)\n", typeName)
		b.WriteString("\t\t\t}\n\n")
		fmt.Fprintf(b, "\t\t\treturn fmt.Errorf(\"%s (%%d) still exists\", id)\n", typeName)
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	} else {
		// Fallback: TODO stub
		fmt.Fprintf(b, "func testAccCheck%sDestroy(_ context.Context) resource.TestCheckFunc {\n", pascal)
		b.WriteString("\treturn func(s *terraform.State) error {\n")
		b.WriteString("\t\tfor _, rs := range s.RootModule().Resources {\n")
		fmt.Fprintf(b, "\t\t\tif rs.Type != %q {\n", typeName)
		b.WriteString("\t\t\t\tcontinue\n")
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t\t// TODO: Call SDK to verify the resource no longer exists.\n")
		b.WriteString("\t\t\t// Return nil if 404, return error if still exists.\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	}
}

type attrInfo struct {
	name     string
	attrType string
}

func getRequiredResourceAttrs(s rsschema.Schema) []attrInfo {
	var attrs []attrInfo
	names := sortedKeys(s.Attributes)
	for _, name := range names {
		attr := s.Attributes[name]
		if isResourceComputedOnly(attr) {
			continue
		}
		if !isResourceRequired(attr) {
			continue
		}
		attrs = append(attrs, attrInfo{
			name:     name,
			attrType: resourceAttrType(attr),
		})
	}
	return attrs
}

func hasNameField(attrs []attrInfo) bool {
	for _, a := range attrs {
		if a.name == "name" {
			return true
		}
	}
	return false
}

func buildBasicConfig(typeName string, requiredAttrs []attrInfo, hasName bool, meta *ResourceMeta) string {
	var b strings.Builder

	// Write dependency resources first
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

	fmt.Fprintf(&b, "resource %q %q {\n", typeName, "test")

	for _, attr := range requiredAttrs {
		val := basicTestValueWithMeta(attr, hasName, meta)
		fmt.Fprintf(&b, "  %s = %s\n", attr.name, val)
	}

	// Write dependency references (fields that reference other resources)
	if meta != nil {
		for _, dep := range meta.Dependencies {
			// Only write if not already in required attrs
			if !attrInList(dep.TargetField, requiredAttrs) {
				fmt.Fprintf(&b, "  %s = %s.%s.%s\n", dep.TargetField, dep.TypeName, dep.RefName, dep.RefAttribute)
			}
		}

		for _, block := range meta.ExtraHCLBlocks {
			fmt.Fprintf(&b, "  %s\n", block)
		}
	}

	b.WriteString("}\n")
	return b.String()
}

func buildUpdateConfig(typeName string, requiredAttrs []attrInfo, s rsschema.Schema, hasName bool, meta *ResourceMeta) string {
	var b strings.Builder

	// Write dependency resources first
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

	fmt.Fprintf(&b, "resource %q %q {\n", typeName, "test")

	for _, attr := range requiredAttrs {
		val := updateTestValueWithMeta(attr, hasName, meta)
		fmt.Fprintf(&b, "  %s = %s\n", attr.name, val)
	}

	// Add one optional attr if available
	optionalAttr := firstOptionalAttr(s)
	if optionalAttr != nil {
		val := updateTestValue(*optionalAttr, false)
		fmt.Fprintf(&b, "  %s = %s\n", optionalAttr.name, val)
	}

	// Write dependency references
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

	b.WriteString("}\n")
	return b.String()
}

// basicTestValueWithMeta returns a domain-valid basic value from the registry,
// falling back to the generic basicTestValue if no override exists.
func basicTestValueWithMeta(attr attrInfo, hasName bool, meta *ResourceMeta) string {
	if meta != nil {
		if fv, ok := meta.FieldOverrides[attr.name]; ok {
			return fv.Basic
		}
	}
	return basicTestValue(attr, hasName)
}

// updateTestValueWithMeta returns a domain-valid update value from the registry,
// falling back to the generic updateTestValue if no override exists.
func updateTestValueWithMeta(attr attrInfo, hasName bool, meta *ResourceMeta) string {
	if meta != nil {
		if fv, ok := meta.FieldOverrides[attr.name]; ok {
			if fv.Update != "" {
				return fv.Update
			}
			return fv.Basic
		}
	}
	return updateTestValue(attr, hasName)
}

func basicTestValue(attr attrInfo, hasName bool) string {
	if attr.name == "name" && hasName {
		return "%[1]q"
	}
	switch attr.attrType {
	case "string":
		if isNameLikeField(attr.name) {
			return `"test-acc-value"`
		}
		return `"test-acc-value"`
	case "int64":
		return "1"
	case "bool":
		return "false"
	case "float64":
		return "1.0"
	case "map":
		return "{}"
	case "list", "set":
		return "[]"
	default:
		return `"test-acc-value"`
	}
}

func updateTestValue(attr attrInfo, hasName bool) string {
	if attr.name == "name" && hasName {
		return "%[1]q"
	}
	switch attr.attrType {
	case "string":
		return `"test-acc-updated"`
	case "int64":
		return "2"
	case "bool":
		return "true"
	case "float64":
		return "2.0"
	case "map":
		return "{\n    key = \"value\"\n  }"
	case "list", "set":
		return "[]"
	default:
		return `"test-acc-updated"`
	}
}

func isNameLikeField(name string) bool {
	return name == "name" || strings.HasSuffix(name, "_name") || name == "key" || name == "title"
}

func firstOptionalAttr(s rsschema.Schema) *attrInfo {
	names := sortedKeys(s.Attributes)
	for _, name := range names {
		attr := s.Attributes[name]
		if isResourceComputedOnly(attr) {
			continue
		}
		if isResourceRequired(attr) {
			continue
		}
		if isResourceOptional(attr) {
			return &attrInfo{
				name:     name,
				attrType: resourceAttrType(attr),
			}
		}
	}
	return nil
}

func resourceAttrType(attr rsschema.Attribute) string {
	switch attr.(type) {
	case rsschema.StringAttribute:
		return "string"
	case rsschema.Int64Attribute:
		return "int64"
	case rsschema.BoolAttribute:
		return "bool"
	case rsschema.Float64Attribute:
		return "float64"
	case rsschema.MapAttribute:
		return "map"
	case rsschema.ListAttribute:
		return "list"
	case rsschema.SetAttribute:
		return "set"
	default:
		return "string"
	}
}

func isResourceRequired(attr rsschema.Attribute) bool {
	switch a := attr.(type) {
	case rsschema.StringAttribute:
		return a.Required
	case rsschema.Int64Attribute:
		return a.Required
	case rsschema.BoolAttribute:
		return a.Required
	case rsschema.Float64Attribute:
		return a.Required
	case rsschema.MapAttribute:
		return a.Required
	case rsschema.ListAttribute:
		return a.Required
	case rsschema.SetAttribute:
		return a.Required
	default:
		return false
	}
}

func isResourceOptional(attr rsschema.Attribute) bool {
	switch a := attr.(type) {
	case rsschema.StringAttribute:
		return a.Optional
	case rsschema.Int64Attribute:
		return a.Optional
	case rsschema.BoolAttribute:
		return a.Optional
	case rsschema.Float64Attribute:
		return a.Optional
	case rsschema.MapAttribute:
		return a.Optional
	case rsschema.ListAttribute:
		return a.Optional
	case rsschema.SetAttribute:
		return a.Optional
	default:
		return false
	}
}

func isResourceComputedOnly(attr rsschema.Attribute) bool {
	switch a := attr.(type) {
	case rsschema.StringAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.Int64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.BoolAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.Float64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.MapAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.ListAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.SetAttribute:
		return a.Computed && !a.Required && !a.Optional
	default:
		return false
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// metaHasFormatVerbs returns true if any field override or dependency field
// contains a fmt.Sprintf format verb (e.g., %[1]s, %[1]q), meaning the
// config function needs an rName parameter.
func metaHasFormatVerbs(meta *ResourceMeta) bool {
	if meta == nil {
		return false
	}
	for _, fv := range meta.FieldOverrides {
		if strings.Contains(fv.Basic, "%[1]") || strings.Contains(fv.Update, "%[1]") {
			return true
		}
	}
	for _, dep := range meta.Dependencies {
		for _, v := range dep.Fields {
			if strings.Contains(v, "%[1]") {
				return true
			}
		}
	}
	return false
}

func attrInList(name string, attrs []attrInfo) bool {
	for _, a := range attrs {
		if a.name == name {
			return true
		}
	}
	return false
}
