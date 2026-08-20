package tests

import (
	"strings"
	"testing"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// schemaWithSDK builds a resource schema resembling kion_label (name + required
// string + optional string + computed id) so the "hasSDK" template branches are
// exercised.
func labelLikeSchema() rsschema.Schema {
	return rsschema.Schema{
		Attributes: map[string]rsschema.Attribute{
			"id":    rsschema.Int64Attribute{Computed: true},
			"name":  rsschema.StringAttribute{Required: true},
			"key":   rsschema.StringAttribute{Required: true},
			"color": rsschema.StringAttribute{Optional: true},
		},
	}
}

// noSDKSchema builds a resource schema for a type NOT in the registry, so the
// TODO-stub fallback branches of buildExistsFunc/buildDestroyFunc are hit.
func noSDKSchema() rsschema.Schema {
	return rsschema.Schema{
		Attributes: map[string]rsschema.Attribute{
			"id":      rsschema.Int64Attribute{Computed: true},
			"enabled": rsschema.BoolAttribute{Required: true},
			"count":   rsschema.Int64Attribute{Optional: true},
		},
	}
}

func TestBuildResourceTestFile_WithSDK(t *testing.T) {
	out := buildResourceTestFile("label", "kion_label", "label", "Label", labelLikeSchema())

	mustContain := []string{
		"package label_test",
		"func TestAccKionLabel_basic(t *testing.T) {",
		"func TestAccKionLabel_update(t *testing.T) {",
		"func testAccCheckLabelExists(",
		"func testAccCheckLabelDestroy(",
		"func testAccLabelConfig_basic(rName string) string {",
		"func testAccLabelConfig_update(rName string) string {",
		// hasSDK -> strconv + errs + generated import
		"\"strconv\"",
		"terraform-provider-kion/internal/errs",
		SDKImportPath,
		// real SDK call from the registry
		"conn.Client.GetLabel(ctx,",
		"errs.IsNotFound(out)",
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("buildResourceTestFile output missing %q", want)
		}
	}
}

func TestBuildResourceTestFile_NoSDK_StubBranch(t *testing.T) {
	out := buildResourceTestFile("thing", "kion_thing", "thing", "Thing", noSDKSchema())

	// Not in the registry -> TODO stub, no strconv/errs/generated imports.
	if strings.Contains(out, "terraform-provider-kion/internal/errs") {
		t.Error("no-SDK resource should not import errs")
	}
	if strings.Contains(out, "conn.Client.") {
		t.Error("no-SDK resource should not emit an SDK client call")
	}
	for _, want := range []string{
		"// TODO: Call SDK to verify the resource exists.",
		"// TODO: Call SDK to verify the resource no longer exists.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected TODO stub %q in no-SDK output", want)
		}
	}
	// No name field and no registry format verbs -> config funcs take no args.
	if !strings.Contains(out, "func testAccThingConfig_basic() string {") {
		t.Error("expected zero-arg basic config func for no-name resource")
	}
}

func TestBuildSweepFile_WithAndWithoutSDK(t *testing.T) {
	// kion_label has a delete method -> the SDK-flavored sweeper.
	withSDK := buildSweepFile("label", "kion_label", "Label")
	for _, want := range []string{
		"package label",
		`resource.AddTestSweepers("kion_label"`,
		"func sweepLabel(_ string) error {",
		"conns.SharedClient()",
		"conn.Client.DeleteLabel()",
	} {
		if !strings.Contains(withSDK, want) {
			t.Errorf("SDK sweep file missing %q", want)
		}
	}

	// A type not in the registry -> the stub sweeper without conns import.
	stub := buildSweepFile("thing", "kion_thing", "Thing")
	if strings.Contains(stub, "conns.SharedClient()") {
		t.Error("stub sweeper should not reference conns.SharedClient")
	}
	if !strings.Contains(stub, "func sweepThing(_ string) error {") {
		t.Error("expected sweepThing stub function")
	}
}

func TestBuildSweepEntrypoint(t *testing.T) {
	out := buildSweepEntrypoint([]string{"label", "cloud_rule"})
	for _, want := range []string{
		"package sweep_test",
		"func TestMain(m *testing.M) {",
		"resource.TestMain(m)",
		`_ "terraform-provider-kion/internal/service/label"`,
		`_ "terraform-provider-kion/internal/service/cloud_rule"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sweep entrypoint missing %q", want)
		}
	}
}

func TestBuildDataSourceTestFile_WithMatchingResource(t *testing.T) {
	dsSchema := dsschema.Schema{
		Attributes: map[string]dsschema.Attribute{
			"id":    dsschema.Int64Attribute{Required: true},
			"name":  dsschema.StringAttribute{Computed: true},
			"color": dsschema.StringAttribute{Computed: true},
		},
	}
	resSchema := labelLikeSchema()

	out := buildDataSourceTestFile("label", "kion_label", "label", "Label", dsSchema, "kion_label", &resSchema)

	for _, want := range []string{
		"package label_test",
		"func TestAccKionLabelDataSource_basic(t *testing.T) {",
		"func testAccLabelDataSourceConfig_basic(rName string) string {",
		// Data source reads back the resource by id.
		"data \"kion_label\" \"test\" {",
		"id = kion_label.test.id",
		// Computed attribute checks.
		`resource.TestCheckResourceAttrSet(dataSourceName, "color")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("data source test file missing %q", want)
		}
	}
}

func TestBuildDataSourceTestFile_NoMatchingResource(t *testing.T) {
	dsSchema := dsschema.Schema{
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.Int64Attribute{Required: true},
		},
	}

	// No resource type / schema -> standalone TODO config.
	out := buildDataSourceTestFile("widget", "kion_widget", "widget", "Widget", dsSchema, "", nil)

	if !strings.Contains(out, "func testAccWidgetDataSourceConfig_basic() string {") {
		t.Error("expected zero-arg standalone data source config func")
	}
	if !strings.Contains(out, "# TODO: Fill in filter criteria or ID to look up the data source.") {
		t.Error("expected standalone TODO placeholder in config")
	}
}

// TestGetRequiredResourceAttrs verifies required (non-computed-only) attrs are
// selected, sorted, and typed, while computed-only and optional attrs are
// excluded.
func TestGetRequiredResourceAttrs(t *testing.T) {
	s := rsschema.Schema{
		Attributes: map[string]rsschema.Attribute{
			"id":      rsschema.Int64Attribute{Computed: true},                  // computed-only -> excluded
			"name":    rsschema.StringAttribute{Required: true},                 // required
			"count":   rsschema.Int64Attribute{Required: true},                  // required
			"enabled": rsschema.BoolAttribute{Optional: true},                   // optional -> excluded
			"rate":    rsschema.Float64Attribute{Required: true},                // required
			"tags":    rsschema.MapAttribute{Required: true},                    // required
			"regions": rsschema.ListAttribute{Required: true},                   // required
			"extra":   rsschema.StringAttribute{Computed: true, Optional: true}, // optional -> excluded
		},
	}

	got := getRequiredResourceAttrs(s)

	// Build a name->type map for easy assertions.
	byName := map[string]string{}
	for _, a := range got {
		byName[a.name] = a.attrType
	}

	wantRequired := map[string]string{
		"name":    "string",
		"count":   "int64",
		"rate":    "float64",
		"tags":    "map",
		"regions": "list",
	}
	for n, ty := range wantRequired {
		if byName[n] != ty {
			t.Errorf("required attr %q type = %q, want %q", n, byName[n], ty)
		}
	}
	for _, excluded := range []string{"id", "enabled", "extra"} {
		if _, ok := byName[excluded]; ok {
			t.Errorf("attr %q should have been excluded from required attrs", excluded)
		}
	}

	// Results must be sorted by name.
	for i := 1; i < len(got); i++ {
		if got[i-1].name > got[i].name {
			t.Errorf("required attrs not sorted: %q before %q", got[i-1].name, got[i].name)
		}
	}
}

// TestFirstOptionalAttr picks the first optional (non-required, non-computed)
// attribute in sorted order.
func TestFirstOptionalAttr(t *testing.T) {
	s := rsschema.Schema{
		Attributes: map[string]rsschema.Attribute{
			"id":    rsschema.Int64Attribute{Computed: true},
			"zeta":  rsschema.StringAttribute{Optional: true},
			"alpha": rsschema.StringAttribute{Required: true},
			"beta":  rsschema.BoolAttribute{Optional: true},
		},
	}
	got := firstOptionalAttr(s)
	if got == nil {
		t.Fatal("firstOptionalAttr returned nil, want an attr")
	}
	// "beta" sorts before "zeta".
	if got.name != "beta" {
		t.Errorf("firstOptionalAttr = %q, want %q", got.name, "beta")
	}

	// A schema with no optional attrs returns nil.
	noOpt := rsschema.Schema{
		Attributes: map[string]rsschema.Attribute{
			"id":   rsschema.Int64Attribute{Computed: true},
			"name": rsschema.StringAttribute{Required: true},
		},
	}
	if got := firstOptionalAttr(noOpt); got != nil {
		t.Errorf("firstOptionalAttr(noOpt) = %+v, want nil", got)
	}
}

// TestResourceAttrTypeMapping covers every branch of resourceAttrType.
func TestResourceAttrTypeMapping(t *testing.T) {
	cases := []struct {
		attr rsschema.Attribute
		want string
	}{
		{rsschema.StringAttribute{}, "string"},
		{rsschema.Int64Attribute{}, "int64"},
		{rsschema.BoolAttribute{}, "bool"},
		{rsschema.Float64Attribute{}, "float64"},
		{rsschema.MapAttribute{}, "map"},
		{rsschema.ListAttribute{}, "list"},
		{rsschema.SetAttribute{}, "set"},
		// Unknown attribute type falls back to "string".
		{rsschema.ObjectAttribute{}, "string"},
	}
	for _, tc := range cases {
		if got := resourceAttrType(tc.attr); got != tc.want {
			t.Errorf("resourceAttrType(%T) = %q, want %q", tc.attr, got, tc.want)
		}
	}
}

// TestIsNameLikeField covers the name-like heuristic.
func TestIsNameLikeField(t *testing.T) {
	cases := map[string]bool{
		"name":         true,
		"display_name": true,
		"key":          true,
		"title":        true,
		"description":  false,
		"color":        false,
	}
	for field, want := range cases {
		if got := isNameLikeField(field); got != want {
			t.Errorf("isNameLikeField(%q) = %v, want %v", field, got, want)
		}
	}
}

// TestBasicAndUpdateTestValue_Collections covers the map/list/set branches that
// the existing tests do not.
func TestBasicAndUpdateTestValue_Collections(t *testing.T) {
	if got := basicTestValue(attrInfo{name: "tags", attrType: "map"}, false); got != "{}" {
		t.Errorf("basicTestValue(map) = %q, want %q", got, "{}")
	}
	if got := basicTestValue(attrInfo{name: "regions", attrType: "list"}, false); got != "[]" {
		t.Errorf("basicTestValue(list) = %q, want %q", got, "[]")
	}
	if got := basicTestValue(attrInfo{name: "regions", attrType: "set"}, false); got != "[]" {
		t.Errorf("basicTestValue(set) = %q, want %q", got, "[]")
	}
	if got := updateTestValue(attrInfo{name: "rate", attrType: "float64"}, false); got != "2.0" {
		t.Errorf("updateTestValue(float64) = %q, want %q", got, "2.0")
	}
	if got := updateTestValue(attrInfo{name: "regions", attrType: "list"}, false); got != "[]" {
		t.Errorf("updateTestValue(list) = %q, want %q", got, "[]")
	}
	// Unknown attr type falls through to the default string value.
	if got := basicTestValue(attrInfo{name: "x", attrType: "object"}, false); got != `"test-acc-value"` {
		t.Errorf("basicTestValue(unknown) = %q, want default", got)
	}
	if got := updateTestValue(attrInfo{name: "x", attrType: "object"}, false); got != `"test-acc-updated"` {
		t.Errorf("updateTestValue(unknown) = %q, want default", got)
	}
}

// TestBuildUpdateConfig_AppendsOptionalAttr checks that an optional schema
// attribute is appended to the update config.
func TestBuildUpdateConfig_AppendsOptionalAttr(t *testing.T) {
	meta := GetMeta("kion_label")
	attrs := []attrInfo{
		{name: "key", attrType: "string"},
		{name: "value", attrType: "string"},
	}
	s := labelLikeSchema()

	out := buildUpdateConfig("kion_label", attrs, s, false, meta)

	if !strings.Contains(out, `resource "kion_label" "test"`) {
		t.Error("expected resource block in update config")
	}
	// "color" is the optional attr; it should be appended.
	if !strings.Contains(out, "color =") {
		t.Errorf("expected optional attr color in update config, got:\n%s", out)
	}
}

// TestAttrInList covers both hit and miss.
func TestAttrInList(t *testing.T) {
	attrs := []attrInfo{{name: "a"}, {name: "b"}}
	if !attrInList("b", attrs) {
		t.Error("attrInList(b) = false, want true")
	}
	if attrInList("z", attrs) {
		t.Error("attrInList(z) = true, want false")
	}
}

// TestHasNameAttrInResourceSchema covers required-name true/false.
func TestHasNameAttrInResourceSchema(t *testing.T) {
	withName := labelLikeSchema()
	if !hasNameAttrInResourceSchema(&withName) {
		t.Error("expected hasNameAttrInResourceSchema true for label-like schema")
	}
	noName := noSDKSchema()
	if hasNameAttrInResourceSchema(&noName) {
		t.Error("expected hasNameAttrInResourceSchema false when name is absent")
	}
	// name present but optional (not required) -> false.
	optName := rsschema.Schema{
		Attributes: map[string]rsschema.Attribute{
			"name": rsschema.StringAttribute{Optional: true},
		},
	}
	if hasNameAttrInResourceSchema(&optName) {
		t.Error("expected false when name is optional, not required")
	}
}
