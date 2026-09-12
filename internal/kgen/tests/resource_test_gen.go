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
	hasRaw := !hasSDK && metaHasRawCollection(meta)
	needsRName := hasName || metaHasFormatVerbs(meta)

	var b strings.Builder

	writeKnownIssues(&b, meta)
	fmt.Fprintf(&b, "package %s_test\n\n", pkgName)
	b.WriteString("import (\n")
	b.WriteString("\t\"context\"\n")
	if hasRaw {
		b.WriteString("\t\"encoding/json\"\n")
	}
	b.WriteString("\t\"fmt\"\n")
	if metaNeedsEnv(meta) {
		b.WriteString("\t\"os\"\n")
	}
	if hasSDK || hasRaw {
		b.WriteString("\t\"strconv\"\n")
	}
	if hasRaw && meta.RawCollectionParentField != "" {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString("\t\"testing\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/helper/resource\"\n")
	b.WriteString("\t\"github.com/hashicorp/terraform-plugin-testing/terraform\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"terraform-provider-kion/internal/acctest\"\n")
	if hasRaw {
		b.WriteString("\t\"terraform-provider-kion/internal/conns\"\n")
		if meta.RawCollectionDiscriminator != "" {
			b.WriteString("\t\"terraform-provider-kion/internal/flex\"\n")
		}
	}
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
	writeEnvSkips(&b, meta)
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
		fmt.Fprintf(&b, "\t\t\t\tConfig: testAcc%sConfig_basic(%s),\n", pascal, configCallArgs(true, meta))
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
	writeImportStep(&b, meta)
	b.WriteString("\t\t},\n")
	b.WriteString("\t})\n")
	b.WriteString("}\n\n")

	// TestAccKion<Name>_update
	hasUpdate := meta == nil || !meta.NoUpdate
	if hasUpdate {
		writeUpdateTest(&b, pascal, typeName, needsRName, meta)
	}

	// rawLookup<Name>, when the record is only visible through a collection
	buildRawLookupFunc(&b, pascal, meta, hasRaw)

	// testAccCheck<Name>Exists
	buildExistsFunc(&b, pascal, typeName, meta)

	// testAccCheck<Name>Destroy
	buildDestroyFunc(&b, pascal, typeName, meta)

	// testAcc<Name>Config_basic
	basicConfig := buildBasicConfig(typeName, requiredAttrs, needsRName, meta)
	envArgs := envArgNames(meta)
	if needsRName || len(envArgs) > 0 {
		fmt.Fprintf(&b, "func testAcc%sConfig_basic(%s) string {\n", pascal, configParams(needsRName, envArgs))
		b.WriteString("\treturn fmt.Sprintf(`\n")
		b.WriteString(basicConfig)
		fmt.Fprintf(&b, "`, %s)\n", configArgs(needsRName, envArgs))
	} else {
		fmt.Fprintf(&b, "func testAcc%sConfig_basic() string {\n", pascal)
		b.WriteString("\treturn `\n")
		b.WriteString(basicConfig)
		b.WriteString("`\n")
	}
	b.WriteString("}\n")

	if !hasUpdate {
		return b.String()
	}
	b.WriteString("\n")

	// testAcc<Name>Config_update
	updateConfig := buildUpdateConfig(typeName, requiredAttrs, s, needsRName, meta)
	if needsRName || len(envArgs) > 0 {
		fmt.Fprintf(&b, "func testAcc%sConfig_update(%s) string {\n", pascal, configParams(needsRName, envArgs))
		b.WriteString("\treturn fmt.Sprintf(`\n")
		b.WriteString(updateConfig)
		fmt.Fprintf(&b, "`, %s)\n", configArgs(needsRName, envArgs))
	} else {
		fmt.Fprintf(&b, "func testAcc%sConfig_update() string {\n", pascal)
		b.WriteString("\treturn `\n")
		b.WriteString(updateConfig)
		b.WriteString("`\n")
	}
	b.WriteString("}\n")

	return b.String()
}

// writeUpdateTest emits TestAccKion<Name>_update: create, change, re-check,
// then import.
func writeUpdateTest(b *strings.Builder, pascal, typeName string, needsRName bool, meta *ResourceMeta) {
	fmt.Fprintf(b, "func TestAccKion%s_update(t *testing.T) {\n", pascal)
	b.WriteString("\tif testing.Short() {\n")
	b.WriteString("\t\tt.Skip(\"skipping long-running test in short mode\")\n")
	b.WriteString("\t}\n\n")
	writeEnvSkips(b, meta)
	b.WriteString("\tctx := acctest.Context(t)\n")
	if needsRName {
		b.WriteString("\trName := acctest.RandomWithPrefix(acctest.ResourcePrefix)\n")
	}
	fmt.Fprintf(b, "\tresourceName := %q\n\n", typeName+".test")
	b.WriteString("\tresource.Test(t, resource.TestCase{\n")
	b.WriteString("\t\tPreCheck:                 func() { acctest.PreCheck(t) },\n")
	b.WriteString("\t\tProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,\n")
	fmt.Fprintf(b, "\t\tCheckDestroy:             testAccCheck%sDestroy(ctx),\n", pascal)
	b.WriteString("\t\tSteps: []resource.TestStep{\n")
	b.WriteString("\t\t\t{\n")
	if needsRName {
		fmt.Fprintf(b, "\t\t\t\tConfig: testAcc%sConfig_basic(rName),\n", pascal)
	} else {
		fmt.Fprintf(b, "\t\t\t\tConfig: testAcc%sConfig_basic(),\n", pascal)
	}
	b.WriteString("\t\t\t\tCheck: resource.ComposeAggregateTestCheckFunc(\n")
	fmt.Fprintf(b, "\t\t\t\t\ttestAccCheck%sExists(ctx, resourceName),\n", pascal)
	b.WriteString("\t\t\t\t\tresource.TestCheckResourceAttrSet(resourceName, \"id\"),\n")
	b.WriteString("\t\t\t\t),\n")
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\t{\n")
	if needsRName {
		fmt.Fprintf(b, "\t\t\t\tConfig: testAcc%sConfig_update(rName),\n", pascal)
	} else {
		fmt.Fprintf(b, "\t\t\t\tConfig: testAcc%sConfig_update(),\n", pascal)
	}
	b.WriteString("\t\t\t\tCheck: resource.ComposeAggregateTestCheckFunc(\n")
	fmt.Fprintf(b, "\t\t\t\t\ttestAccCheck%sExists(ctx, resourceName),\n", pascal)
	b.WriteString("\t\t\t\t\tresource.TestCheckResourceAttrSet(resourceName, \"id\"),\n")
	b.WriteString("\t\t\t\t),\n")
	b.WriteString("\t\t\t},\n")
	writeImportStep(b, meta)
	b.WriteString("\t\t},\n")
	b.WriteString("\t})\n")
	b.WriteString("}\n\n")
}

// writeImportStep emits the ImportState test step. A resource whose
// ImportState parses "<parent>/<id>" gets an ImportStateIdFunc that rebuilds
// that form from state; everything else imports by its bare id.
func writeImportStep(b *strings.Builder, meta *ResourceMeta) {
	b.WriteString("\t\t\t{\n")
	b.WriteString("\t\t\t\tResourceName:      resourceName,\n")
	b.WriteString("\t\t\t\tImportState:       true,\n")
	b.WriteString("\t\t\t\tImportStateVerify: true,\n")
	if meta != nil && meta.ImportIDParentField != "" {
		b.WriteString("\t\t\t\tImportStateIdFunc: func(s *terraform.State) (string, error) {\n")
		b.WriteString("\t\t\t\t\trs, ok := s.RootModule().Resources[resourceName]\n")
		b.WriteString("\t\t\t\t\tif !ok {\n")
		b.WriteString("\t\t\t\t\t\treturn \"\", fmt.Errorf(\"not found: %s\", resourceName)\n")
		b.WriteString("\t\t\t\t\t}\n")
		fmt.Fprintf(b, "\t\t\t\t\treturn rs.Primary.Attributes[%q] + \"/\" + rs.Primary.ID, nil\n", meta.ImportIDParentField)
		b.WriteString("\t\t\t\t},\n")
	}
	b.WriteString("\t\t\t},\n")
}

// metaNeedsEnv reports whether the generated file reads os.Getenv.
func metaNeedsEnv(meta *ResourceMeta) bool {
	return meta != nil && len(meta.RequiredEnv) > 0
}

// writeKnownIssues emits the registry's KnownIssues as a file header comment,
// so a test that is red on purpose says which defect it is holding open.
func writeKnownIssues(b *strings.Builder, meta *ResourceMeta) {
	if meta == nil || len(meta.KnownIssues) == 0 {
		return
	}
	b.WriteString("// Known issues this test is expected to surface:\n")
	for _, issue := range meta.KnownIssues {
		fmt.Fprintf(b, "//   %s\n", issue)
	}
	b.WriteString("\n")
}

// writeEnvSkips emits one skip guard per required environment variable. A
// missing id is reported as SKIP naming the variable, never as a pass.
func writeEnvSkips(b *strings.Builder, meta *ResourceMeta) {
	if meta == nil {
		return
	}
	for _, e := range meta.RequiredEnv {
		fmt.Fprintf(b, "\tif os.Getenv(%q) == \"\" {\n", e.Name)
		fmt.Fprintf(b, "\t\tt.Skip(%q)\n", e.Name+" must be set to "+e.Reason)
		b.WriteString("\t}\n\n")
	}
}

// metaHasRawCollection reports whether the resource is read back through a
// private collection rather than a single-record SDK get.
func metaHasRawCollection(meta *ResourceMeta) bool {
	return meta != nil && meta.RawCollectionPath != ""
}

// rawLookupArgs is the argument list the generated rawLookup<Name> takes: the
// parent id first when the collection is parent-scoped, then the record id.
func rawLookupArgs(meta *ResourceMeta) string {
	if meta.RawCollectionParentField != "" {
		return "parentID, id string"
	}
	return "id string"
}

// buildRawLookupFunc emits rawLookup<Name>, which answers whether the private
// collection the resource is read through still holds a given id. It is what
// the Exists and Destroy checks call for a resource with no single-record GET,
// replacing a TODO stub that returned nil however the API had behaved.
func buildRawLookupFunc(b *strings.Builder, pascal string, meta *ResourceMeta, hasRaw bool) {
	if !hasRaw {
		return
	}

	fmt.Fprintf(b, "// rawLookup%s reports whether the private collection this resource is read\n", pascal)
	b.WriteString("// through still holds the given id. There is no single-record GET, so the\n")
	b.WriteString("// collection is the only way to see the record.\n")
	fmt.Fprintf(b, "func rawLookup%s(%s) (bool, error) {\n", pascal, rawLookupArgs(meta))
	b.WriteString("\tconn, err := acctest.SharedClient()\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn false, fmt.Errorf(\"getting shared client: %w\", err)\n")
	b.WriteString("\t}\n\n")
	b.WriteString("\twant, err := strconv.ParseInt(id, 10, 64)\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn false, fmt.Errorf(\"parsing ID %q: %w\", id, err)\n")
	b.WriteString("\t}\n\n")
	if meta.RawCollectionParentField != "" {
		fmt.Fprintf(b, "\tpath := strings.Replace(%q, \"{parent}\", parentID, 1)\n", meta.RawCollectionPath)
	} else {
		fmt.Fprintf(b, "\tpath := %q\n", meta.RawCollectionPath)
	}
	b.WriteString("\tbody, err := conn.RawGet(context.Background(), path)\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\tif conns.IsRawNotFound(err) {\n")
	b.WriteString("\t\t\treturn false, nil\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\treturn false, fmt.Errorf(\"reading %s: %w\", path, err)\n")
	b.WriteString("\t}\n\n")
	b.WriteString("\tvar env struct {\n")
	b.WriteString("\t\tData []struct {\n")
	// Field widths are padded to what gofmt would produce: the generator writes
	// the file verbatim, so a misaligned struct tag is a gofmt failure in CI.
	if meta.RawCollectionDiscriminator != "" {
		b.WriteString("\t\t\tID   int64         `json:\"id\"`\n")
		fmt.Fprintf(b, "\t\t\tKind *flex.NullInt `json:%q`\n", meta.RawCollectionDiscriminator)
	} else {
		b.WriteString("\t\t\tID int64 `json:\"id\"`\n")
	}
	b.WriteString("\t\t} `json:\"data\"`\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif err := json.Unmarshal(body, &env); err != nil {\n")
	b.WriteString("\t\treturn false, fmt.Errorf(\"decoding %s: %w\", path, err)\n")
	b.WriteString("\t}\n\n")
	b.WriteString("\tfor _, rec := range env.Data {\n")
	b.WriteString("\t\tif rec.ID != want {\n")
	b.WriteString("\t\t\tcontinue\n")
	b.WriteString("\t\t}\n")
	if meta.RawCollectionDiscriminator != "" {
		b.WriteString("\t\t// The collection mixes in a neighboring kind's records; only those\n")
		fmt.Fprintf(b, "\t\t// carrying a valid %s are this resource.\n", meta.RawCollectionDiscriminator)
		b.WriteString("\t\tif rec.Kind == nil || !rec.Kind.Valid {\n")
		b.WriteString("\t\t\tcontinue\n")
		b.WriteString("\t\t}\n")
	}
	b.WriteString("\t\treturn true, nil\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn false, nil\n")
	b.WriteString("}\n\n")
}

// rawLookupCall renders the call to rawLookup<Name> for a state resource held
// in the variable rs.
func rawLookupCall(pascal string, meta *ResourceMeta) string {
	if meta.RawCollectionParentField != "" {
		return fmt.Sprintf("rawLookup%s(rs.Primary.Attributes[%q], rs.Primary.ID)", pascal, meta.RawCollectionParentField)
	}
	return fmt.Sprintf("rawLookup%s(rs.Primary.ID)", pascal)
}

// buildExistsFunc generates the testAccCheck<Name>Exists function.
// When SDK metadata is available, it produces a real API call; otherwise a TODO stub.
func buildExistsFunc(b *strings.Builder, pascal, typeName string, meta *ResourceMeta) {
	if meta != nil && meta.SDKGetMethod == "" && meta.RawCollectionPath != "" {
		fmt.Fprintf(b, "func testAccCheck%sExists(_ context.Context, name string) resource.TestCheckFunc {\n", pascal)
		b.WriteString("\treturn func(s *terraform.State) error {\n")
		b.WriteString("\t\trs, ok := s.RootModule().Resources[name]\n")
		b.WriteString("\t\tif !ok {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"not found: %s\", name)\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\tif rs.Primary.ID == \"\" {\n")
		b.WriteString("\t\t\treturn fmt.Errorf(\"no ID set for %s\", name)\n")
		b.WriteString("\t\t}\n\n")
		fmt.Fprintf(b, "\t\tfound, err := %s\n", rawLookupCall(pascal, meta))
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn err\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\tif !found {\n")
		fmt.Fprintf(b, "\t\t\treturn fmt.Errorf(\"%s (%%s) not found\", rs.Primary.ID)\n", typeName)
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
		return
	}
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
	if meta != nil && meta.SDKGetMethod == "" && meta.RawCollectionPath != "" {
		fmt.Fprintf(b, "func testAccCheck%sDestroy(_ context.Context) resource.TestCheckFunc {\n", pascal)
		b.WriteString("\treturn func(s *terraform.State) error {\n")
		b.WriteString("\t\tfor _, rs := range s.RootModule().Resources {\n")
		fmt.Fprintf(b, "\t\t\tif rs.Type != %q {\n", typeName)
		b.WriteString("\t\t\t\tcontinue\n")
		b.WriteString("\t\t\t}\n\n")
		fmt.Fprintf(b, "\t\t\tfound, err := %s\n", rawLookupCall(pascal, meta))
		b.WriteString("\t\t\tif err != nil {\n")
		b.WriteString("\t\t\t\treturn err\n")
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t\tif found {\n")
		fmt.Fprintf(b, "\t\t\t\treturn fmt.Errorf(\"%s (%%s) still exists\", rs.Primary.ID)\n", typeName)
		b.WriteString("\t\t\t}\n")
		b.WriteString("\t\t}\n\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
		return
	}
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

	deps := depByTargetField(meta)
	for _, attr := range requiredAttrs {
		if ref, ok := deps[attr.name]; ok {
			fmt.Fprintf(&b, "  %s = %s\n", attr.name, ref)
			continue
		}
		val := basicTestValueWithMeta(attr, hasName, meta)
		fmt.Fprintf(&b, "  %s = %s\n", attr.name, val)
	}

	// Write dependency references (fields that reference other resources)
	if meta != nil {
		for _, dep := range meta.Dependencies {
			// Required target fields were emitted as references above.
			if !attrInList(dep.TargetField, requiredAttrs) {
				fmt.Fprintf(&b, "  %s = %s.%s.%s\n", dep.TargetField, dep.TypeName, dep.RefName, dep.RefAttribute)
			}
		}

		// A FieldOverride on an attribute the schema marks Optional is a
		// deliberate "the API needs this anyway" -- kion_category's payer_id is
		// optional in the spec and rejected when absent. Required attributes
		// were already emitted above.
		for _, name := range sortedOverrideNames(meta) {
			if attrInList(name, requiredAttrs) {
				continue
			}
			if _, isDep := deps[name]; isDep {
				continue
			}
			fmt.Fprintf(&b, "  %s = %s\n", name, meta.FieldOverrides[name].Basic)
		}

		for _, block := range meta.ExtraHCLBlocks {
			fmt.Fprintf(&b, "  %s\n", block)
		}
	}

	b.WriteString("}\n")
	return b.String()
}

// sortedOverrideNames lists FieldOverrides keys in a stable order, so the
// generated config does not churn between runs.
func sortedOverrideNames(meta *ResourceMeta) []string {
	if meta == nil {
		return nil
	}
	out := make([]string, 0, len(meta.FieldOverrides))
	for k := range meta.FieldOverrides {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// depByTargetField maps each dependency's target field to the HCL reference
// that resolves it. A target field that is also Required used to be skipped
// here and filled from basicTestValue instead, so the dependency resource was
// created and then never referenced: kion_compliance_family pointed its
// required compliance_program_id at a hard-coded 1 and create returned 404.
func depByTargetField(meta *ResourceMeta) map[string]string {
	if meta == nil {
		return nil
	}
	out := make(map[string]string, len(meta.Dependencies))
	for _, dep := range meta.Dependencies {
		out[dep.TargetField] = fmt.Sprintf("%s.%s.%s", dep.TypeName, dep.RefName, dep.RefAttribute)
	}
	return out
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

	deps := depByTargetField(meta)
	for _, attr := range requiredAttrs {
		if ref, ok := deps[attr.name]; ok {
			fmt.Fprintf(&b, "  %s = %s\n", attr.name, ref)
			continue
		}
		val := updateTestValueWithMeta(attr, hasName, meta)
		fmt.Fprintf(&b, "  %s = %s\n", attr.name, val)
	}

	// Add one optional attr, so the update step actually changes something.
	// It must be one nothing else in this block already writes: picking a
	// dependency's target field or an ExtraHCLBlocks line emits the attribute
	// twice and Terraform rejects the config with "Attribute redefined".
	optionalAttr := firstOptionalAttr(s, alreadyWritten(meta))
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

// alreadyWritten returns the attribute names the resource block emits from the
// registry: each dependency's target field, and the left-hand side of each
// ExtraHCLBlocks line.
func alreadyWritten(meta *ResourceMeta) map[string]bool {
	out := map[string]bool{}
	if meta == nil {
		return out
	}
	for _, dep := range meta.Dependencies {
		out[dep.TargetField] = true
	}
	for _, line := range meta.ExtraHCLBlocks {
		if lhs, _, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(lhs)] = true
		}
	}
	return out
}

func firstOptionalAttr(s rsschema.Schema, skip map[string]bool) *attrInfo {
	names := sortedKeys(s.Attributes)
	for _, name := range names {
		if skip[name] {
			continue
		}
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
	// ExtraHCLBlocks are emitted into the same string. A resource whose every
	// attribute is Optional carries its only values here, so a verb left
	// unscanned reached the config as the literal text "%[1]s".
	for _, line := range meta.ExtraHCLBlocks {
		if strings.Contains(line, "%[1]") {
			return true
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

// envArgNames returns the Go parameter names for a resource's EnvArgs, in
// declaration order. They land as %[2]s, %[3]s, ... in the config template.
func envArgNames(meta *ResourceMeta) []string {
	if meta == nil {
		return nil
	}
	out := make([]string, 0, len(meta.EnvArgs))
	for _, e := range meta.EnvArgs {
		out = append(out, lowerCamelFromEnv(e))
	}
	return out
}

// lowerCamelFromEnv turns KION_ACC_BILLING_SOURCE_ID into billingSourceID.
func lowerCamelFromEnv(name string) string {
	s := strings.TrimPrefix(name, "KION_ACC_")
	parts := strings.Split(strings.ToLower(s), "_")
	for i, p := range parts {
		if i == 0 || p == "" {
			continue
		}
		if p == "id" {
			parts[i] = "ID"
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// configParams renders the parameter list for a generated config function.
func configParams(needsRName bool, args []string) string {
	var ps []string
	if needsRName {
		ps = append(ps, "rName string")
	}
	for _, a := range args {
		ps = append(ps, a+" string")
	}
	return strings.Join(ps, ", ")
}

// configArgs renders the fmt.Sprintf argument list.
func configArgs(needsRName bool, args []string) string {
	var as []string
	if needsRName {
		as = append(as, "rName")
	}
	as = append(as, args...)
	return strings.Join(as, ", ")
}

// configCallArgs renders a call site, reading each EnvArg from the environment.
func configCallArgs(needsRName bool, meta *ResourceMeta) string {
	var as []string
	if needsRName {
		as = append(as, "rName")
	}
	if meta != nil {
		for _, e := range meta.EnvArgs {
			as = append(as, fmt.Sprintf("os.Getenv(%q)", e))
		}
	}
	return strings.Join(as, ", ")
}
