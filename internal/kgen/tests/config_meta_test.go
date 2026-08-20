package tests

import (
	"strings"
	"testing"
)

// TestSDKImportPath asserts the pinned SDK import path constant. This constant
// is flipped per release branch, so a broken value silently redirects every
// generated test file to the wrong SDK sub-package.
func TestSDKImportPath(t *testing.T) {
	const wantPrefix = "github.com/kionsoftware/kion-sdk-go/generated/"
	if !strings.HasPrefix(SDKImportPath, wantPrefix) {
		t.Fatalf("SDKImportPath = %q, want prefix %q", SDKImportPath, wantPrefix)
	}

	// The suffix must be a versioned sub-package (v3_NN).
	suffix := strings.TrimPrefix(SDKImportPath, wantPrefix)
	if !strings.HasPrefix(suffix, "v3_") {
		t.Errorf("SDKImportPath suffix = %q, want a v3_* sub-package", suffix)
	}
	if strings.Contains(suffix, "/") {
		t.Errorf("SDKImportPath suffix = %q, want a single sub-package segment", suffix)
	}
}

// TestProviderTypeName pins the provider type name used when calling Metadata()
// on resources/data sources; every generated type name derives from it.
func TestProviderTypeName(t *testing.T) {
	if providerTypeName != "kion" {
		t.Errorf("providerTypeName = %q, want %q", providerTypeName, "kion")
	}
}

// TestGetMeta_TableKnown exercises several registry entries to confirm their
// SDK method/param metadata is populated as expected.
func TestGetMeta_TableKnown(t *testing.T) {
	cases := []struct {
		typeName        string
		wantGetMethod   string
		wantGetParams   string
		wantDeleteMeth  string
		wantHasOverride bool
	}{
		{
			typeName:        "kion_label",
			wantGetMethod:   "GetLabel",
			wantGetParams:   "generated.GetLabelParams{ID: id}",
			wantDeleteMeth:  "DeleteLabel",
			wantHasOverride: true,
		},
		{
			typeName:        "kion_idms",
			wantGetMethod:   "GetIDMS",
			wantGetParams:   "generated.GetIDMSParams{ID: id}",
			wantDeleteMeth:  "DeleteIDMS",
			wantHasOverride: true,
		},
		{
			typeName:        "kion_permission_scheme",
			wantGetMethod:   "GetPermissionScheme",
			wantGetParams:   "generated.GetPermissionSchemeParams{ID: id}",
			wantDeleteMeth:  "DeletePermissionScheme",
			wantHasOverride: true,
		},
		{
			typeName:        "kion_cloud_rule",
			wantGetMethod:   "GetCloudRuleShow",
			wantGetParams:   "generated.GetCloudRuleShowParams{ID: id}",
			wantDeleteMeth:  "DeleteCloudRule",
			wantHasOverride: true,
		},
		{
			// kion_category intentionally has no single-get endpoint.
			typeName:        "kion_category",
			wantGetMethod:   "",
			wantGetParams:   "",
			wantDeleteMeth:  "DeleteCategoryByID",
			wantHasOverride: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			meta := GetMeta(tc.typeName)
			if meta == nil {
				t.Fatalf("GetMeta(%q) returned nil", tc.typeName)
			}
			if meta.TypeName != tc.typeName {
				t.Errorf("TypeName = %q, want %q", meta.TypeName, tc.typeName)
			}
			if meta.SDKGetMethod != tc.wantGetMethod {
				t.Errorf("SDKGetMethod = %q, want %q", meta.SDKGetMethod, tc.wantGetMethod)
			}
			if meta.SDKGetParams != tc.wantGetParams {
				t.Errorf("SDKGetParams = %q, want %q", meta.SDKGetParams, tc.wantGetParams)
			}
			if meta.SDKDeleteMethod != tc.wantDeleteMeth {
				t.Errorf("SDKDeleteMethod = %q, want %q", meta.SDKDeleteMethod, tc.wantDeleteMeth)
			}
			if tc.wantHasOverride && len(meta.FieldOverrides) == 0 {
				t.Errorf("expected field overrides for %q", tc.typeName)
			}
		})
	}
}

// TestGetMeta_ReturnsCopy verifies GetMeta returns a pointer to a copy, so
// callers cannot mutate the shared registry through the returned pointer.
func TestGetMeta_ReturnsCopy(t *testing.T) {
	m1 := GetMeta("kion_label")
	if m1 == nil {
		t.Fatal("GetMeta(kion_label) returned nil")
	}
	m1.SDKGetMethod = "MutatedByTest"

	m2 := GetMeta("kion_label")
	if m2 == nil {
		t.Fatal("GetMeta(kion_label) returned nil on second call")
	}
	if m2.SDKGetMethod != "GetLabel" {
		t.Errorf("registry was mutated through returned pointer: SDKGetMethod = %q, want %q", m2.SDKGetMethod, "GetLabel")
	}
}

// TestGetMeta_UnknownTable checks several unknown type names all return nil.
func TestGetMeta_UnknownTable(t *testing.T) {
	for _, name := range []string{"", "kion_", "label", "kion_does_not_exist", "KION_LABEL"} {
		if meta := GetMeta(name); meta != nil {
			t.Errorf("GetMeta(%q) = %+v, want nil", name, meta)
		}
	}
}

// TestRegistryEntriesSelfConsistent walks every registry entry and asserts
// invariants the generators rely on.
func TestRegistryEntriesSelfConsistent(t *testing.T) {
	for key, meta := range registry {
		// The map key must match the entry's own TypeName field.
		if meta.TypeName != key {
			t.Errorf("registry[%q].TypeName = %q, want %q", key, meta.TypeName, key)
		}
		// Every type name must carry the kion_ prefix.
		if !strings.HasPrefix(key, "kion_") {
			t.Errorf("registry key %q missing kion_ prefix", key)
		}
		// A get method requires get params, and vice versa.
		if (meta.SDKGetMethod == "") != (meta.SDKGetParams == "") {
			t.Errorf("registry[%q]: SDKGetMethod=%q and SDKGetParams=%q must both be set or both empty", key, meta.SDKGetMethod, meta.SDKGetParams)
		}
		// A delete method requires delete params, and vice versa.
		if (meta.SDKDeleteMethod == "") != (meta.SDKDeleteParams == "") {
			t.Errorf("registry[%q]: SDKDeleteMethod=%q and SDKDeleteParams=%q must both be set or both empty", key, meta.SDKDeleteMethod, meta.SDKDeleteParams)
		}
		// Dependencies must be internally complete.
		for i, dep := range meta.Dependencies {
			if dep.TypeName == "" || dep.RefName == "" || dep.TargetField == "" || dep.RefAttribute == "" {
				t.Errorf("registry[%q].Dependencies[%d] is incomplete: %+v", key, i, dep)
			}
		}
	}
}

// TestGetMeta_ProjectDependencyChain confirms the multi-field dependency on
// kion_project is wired correctly (a Tier-1 entry with an OU dependency).
func TestGetMeta_ProjectDependencyChain(t *testing.T) {
	meta := GetMeta("kion_project")
	if meta == nil {
		t.Fatal("GetMeta(kion_project) returned nil")
	}
	if len(meta.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(meta.Dependencies))
	}
	dep := meta.Dependencies[0]
	if dep.TypeName != "kion_ou" {
		t.Errorf("dep TypeName = %q, want %q", dep.TypeName, "kion_ou")
	}
	if dep.TargetField != "ou_id" {
		t.Errorf("dep TargetField = %q, want %q", dep.TargetField, "ou_id")
	}
	if dep.RefAttribute != "id" {
		t.Errorf("dep RefAttribute = %q, want %q", dep.RefAttribute, "id")
	}
	// The dependency uses a format verb, so the config needs an rName param.
	if !metaHasFormatVerbs(meta) {
		t.Error("expected metaHasFormatVerbs(kion_project) to be true")
	}
}

// TestMetaHasFormatVerbs covers both the field-override and dependency-field
// branches, plus the nil and negative cases.
func TestMetaHasFormatVerbs(t *testing.T) {
	if metaHasFormatVerbs(nil) {
		t.Error("metaHasFormatVerbs(nil) = true, want false")
	}

	// Field-override branch: kion_label has %[1]s in an override.
	if !metaHasFormatVerbs(GetMeta("kion_label")) {
		t.Error("metaHasFormatVerbs(kion_label) = false, want true (override verb)")
	}

	// Dependency-field branch: kion_user_group's idms dep uses %[1]s.
	if !metaHasFormatVerbs(GetMeta("kion_user_group")) {
		t.Error("metaHasFormatVerbs(kion_user_group) = false, want true (dependency verb)")
	}

	// Negative case: an entry with no format verbs anywhere.
	noVerbs := &ResourceMeta{
		TypeName: "kion_synthetic",
		FieldOverrides: map[string]FieldValue{
			"color": {Basic: `"#0088ff"`, Update: `"#ff0000"`},
		},
	}
	if metaHasFormatVerbs(noVerbs) {
		t.Error("metaHasFormatVerbs(noVerbs) = true, want false")
	}
}
