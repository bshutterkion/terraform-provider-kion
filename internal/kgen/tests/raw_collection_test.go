package tests

import (
	"strings"
	"testing"
)

// A resource with no single-record GET used to get Exists and Destroy checks
// whose bodies were `// TODO: Call SDK to verify…` followed by `return nil`.
// Those report green whether or not the API ever saw the record, which is worse
// than having no test at all. RawCollectionPath points the checks at the same
// private collection the resource's own Read uses.

func TestBuildResourceTestFile_RawCollection_FlatPath(t *testing.T) {
	// Not parallel: mutates the package-level registry.
	registry["kion_tag"] = ResourceMeta{
		TypeName:          "kion_tag",
		RawCollectionPath: "/v3/tag",
		NoUpdate:          true,
	}
	t.Cleanup(func() { delete(registry, "kion_tag") })

	out := buildResourceTestFile("tag", "kion_tag", "tag", "Tag", labelLikeSchema())

	for _, want := range []string{
		"\t\"encoding/json\"\n",
		"\t\"terraform-provider-kion/internal/conns\"\n",
		"func rawLookupTag(id string) (bool, error) {",
		`path := "/v3/tag"`,
		"conn.RawGet(context.Background(), path)",
		"conns.IsRawNotFound(err)",
		"found, err := rawLookupTag(rs.Primary.ID)",
		`return fmt.Errorf("kion_tag (%s) not found", rs.Primary.ID)`,
		`return fmt.Errorf("kion_tag (%s) still exists", rs.Primary.ID)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated test file missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"// TODO: Call SDK to verify the resource exists.",
		"// TODO: Call SDK to verify the resource no longer exists.",
		"\t\"strings\"\n",
		"flex.NullInt",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("generated test file still contains %q", unwanted)
		}
	}
}

func TestBuildResourceTestFile_RawCollection_ParentScopedAndDiscriminated(t *testing.T) {
	// Not parallel: mutates the package-level registry.
	registry["kion_exemption"] = ResourceMeta{
		TypeName:                   "kion_exemption",
		RawCollectionPath:          "/v1/ou/{parent}/exemption",
		RawCollectionParentField:   "ou_id",
		RawCollectionDiscriminator: "role_id",
		NoUpdate:                   true,
	}
	t.Cleanup(func() { delete(registry, "kion_exemption") })

	out := buildResourceTestFile("exemption", "kion_exemption", "exemption", "Exemption", labelLikeSchema())

	for _, want := range []string{
		"\t\"strings\"\n",
		"\t\"terraform-provider-kion/internal/flex\"\n",
		"func rawLookupExemption(parentID, id string) (bool, error) {",
		`strings.Replace("/v1/ou/{parent}/exemption", "{parent}", parentID, 1)`,
		"Kind *flex.NullInt `json:\"role_id\"`",
		"if rec.Kind == nil || !rec.Kind.Valid {",
		`rawLookupExemption(rs.Primary.Attributes["ou_id"], rs.Primary.ID)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated test file missing %q", want)
		}
	}
}

// An SDK get method wins: it is a single-record read, which is a stronger check
// than scanning a collection.
func TestBuildResourceTestFile_SDKGetWinsOverRawCollection(t *testing.T) {
	// Not parallel: mutates the package-level registry.
	registry["kion_both"] = ResourceMeta{
		TypeName:          "kion_both",
		SDKGetMethod:      "GetBoth",
		SDKGetParams:      "generated.GetBothParams{ID: id}",
		RawCollectionPath: "/v3/both",
		NoUpdate:          true,
	}
	t.Cleanup(func() { delete(registry, "kion_both") })

	out := buildResourceTestFile("both", "kion_both", "both", "Both", labelLikeSchema())

	if !strings.Contains(out, "conn.Client.GetBoth(ctx, generated.GetBothParams{ID: id})") {
		t.Error("generated test file does not use the SDK get method")
	}
	if strings.Contains(out, "func rawLookupBoth(") {
		t.Error("generated test file emitted a raw lookup alongside an SDK get method")
	}
}

// ExtraHCLBlocks land in the same fmt.Sprintf string as the rest of the config.
// A resource whose every attribute is Optional carries its only values there, so
// a verb left unscanned reached the config as the literal text "%[1]s".
func TestMetaHasFormatVerbs_ScansExtraHCLBlocks(t *testing.T) {
	t.Parallel()

	meta := &ResourceMeta{
		TypeName:       "kion_tag",
		ExtraHCLBlocks: []string{`resource_key = "test-acc-%[1]s"`},
	}
	if !metaHasFormatVerbs(meta) {
		t.Error("a format verb in ExtraHCLBlocks was not detected")
	}
	if metaHasFormatVerbs(&ResourceMeta{ExtraHCLBlocks: []string{`key = "fixed"`}}) {
		t.Error("a literal ExtraHCLBlocks line was reported as carrying a verb")
	}
}
