package tests

import (
	"strings"
	"testing"
)

func TestPascalName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"cloud_rule", "CloudRule"},
		{"label", "Label"},
		{"iam_policy", "IamPolicy"},
		{"gcp_iam_role", "GcpIamRole"},
		{"ou", "Ou"},
		{"cft", "Cft"},
	}
	for _, tt := range tests {
		got := pascalName(tt.input)
		if got != tt.want {
			t.Errorf("pascalName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSnakeNameFromTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"kion_cloud_rule", "cloud_rule"},
		{"kion_label", "label"},
		{"kion_iam_policy", "iam_policy"},
	}
	for _, tt := range tests {
		got := snakeNameFromTypeName(tt.input)
		if got != tt.want {
			t.Errorf("snakeNameFromTypeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBasicTestValue(t *testing.T) {
	tests := []struct {
		attr    attrInfo
		hasName bool
		want    string
	}{
		{attrInfo{name: "name", attrType: "string"}, true, "%[1]q"},
		{attrInfo{name: "description", attrType: "string"}, false, `"test-acc-value"`},
		{attrInfo{name: "count", attrType: "int64"}, false, "1"},
		{attrInfo{name: "enabled", attrType: "bool"}, false, "false"},
		{attrInfo{name: "rate", attrType: "float64"}, false, "1.0"},
	}
	for _, tt := range tests {
		got := basicTestValue(tt.attr, tt.hasName)
		if got != tt.want {
			t.Errorf("basicTestValue(%+v, %v) = %q, want %q", tt.attr, tt.hasName, got, tt.want)
		}
	}
}

func TestUpdateTestValue(t *testing.T) {
	tests := []struct {
		attr    attrInfo
		hasName bool
		want    string
	}{
		{attrInfo{name: "name", attrType: "string"}, true, "%[1]q"},
		{attrInfo{name: "description", attrType: "string"}, false, `"test-acc-updated"`},
		{attrInfo{name: "count", attrType: "int64"}, false, "2"},
		{attrInfo{name: "enabled", attrType: "bool"}, false, "true"},
	}
	for _, tt := range tests {
		got := updateTestValue(tt.attr, tt.hasName)
		if got != tt.want {
			t.Errorf("updateTestValue(%+v, %v) = %q, want %q", tt.attr, tt.hasName, got, tt.want)
		}
	}
}

func TestGetMeta_Known(t *testing.T) {
	meta := GetMeta("kion_label")
	if meta == nil {
		t.Fatal("GetMeta(kion_label) returned nil")
	}
	if meta.SDKGetMethod != "GetLabel" {
		t.Errorf("SDKGetMethod = %q, want %q", meta.SDKGetMethod, "GetLabel")
	}
	if meta.SDKDeleteMethod != "DeleteLabel" {
		t.Errorf("SDKDeleteMethod = %q, want %q", meta.SDKDeleteMethod, "DeleteLabel")
	}
	if len(meta.FieldOverrides) == 0 {
		t.Error("expected field overrides for kion_label")
	}
	if fv, ok := meta.FieldOverrides["color"]; !ok {
		t.Error("expected color field override")
	} else if fv.Basic != `"#0088ff"` {
		t.Errorf("color basic = %q, want %q", fv.Basic, `"#0088ff"`)
	}
}

func TestGetMeta_Unknown(t *testing.T) {
	meta := GetMeta("kion_nonexistent")
	if meta != nil {
		t.Errorf("GetMeta(kion_nonexistent) returned non-nil: %+v", meta)
	}
}

func TestGetMeta_WithDependencies(t *testing.T) {
	meta := GetMeta("kion_ou")
	if meta == nil {
		t.Fatal("GetMeta(kion_ou) returned nil")
	}
	if len(meta.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(meta.Dependencies))
	}
	dep := meta.Dependencies[0]
	if dep.TypeName != "kion_permission_scheme" {
		t.Errorf("dep TypeName = %q, want %q", dep.TypeName, "kion_permission_scheme")
	}
	if dep.TargetField != "permission_scheme_id" {
		t.Errorf("dep TargetField = %q, want %q", dep.TargetField, "permission_scheme_id")
	}
}

func TestGetMeta_WithExtraHCLBlocks(t *testing.T) {
	meta := GetMeta("kion_cloud_rule")
	if meta == nil {
		t.Fatal("GetMeta(kion_cloud_rule) returned nil")
	}
	if len(meta.ExtraHCLBlocks) != 1 {
		t.Fatalf("expected 1 extra HCL block, got %d", len(meta.ExtraHCLBlocks))
	}
	if meta.ExtraHCLBlocks[0] != "owner_users { id = 1 }" {
		t.Errorf("extra block = %q, want %q", meta.ExtraHCLBlocks[0], "owner_users { id = 1 }")
	}
}

func TestBasicTestValueWithMeta(t *testing.T) {
	meta := GetMeta("kion_label")
	if meta == nil {
		t.Fatal("GetMeta(kion_label) returned nil")
	}

	// color has a registry override
	got := basicTestValueWithMeta(attrInfo{name: "color", attrType: "string"}, true, meta)
	if got != `"#0088ff"` {
		t.Errorf("basicTestValueWithMeta(color) = %q, want %q", got, `"#0088ff"`)
	}

	// unknown field falls back to generic
	got = basicTestValueWithMeta(attrInfo{name: "unknown", attrType: "string"}, false, nil)
	if got != `"test-acc-value"` {
		t.Errorf("basicTestValueWithMeta(unknown) = %q, want %q", got, `"test-acc-value"`)
	}
}

func TestUpdateTestValueWithMeta(t *testing.T) {
	meta := GetMeta("kion_label")
	if meta == nil {
		t.Fatal("GetMeta(kion_label) returned nil")
	}

	// color has an update override
	got := updateTestValueWithMeta(attrInfo{name: "color", attrType: "string"}, true, meta)
	if got != `"#ff0000"` {
		t.Errorf("updateTestValueWithMeta(color) = %q, want %q", got, `"#ff0000"`)
	}

	// idms password_expiration has no update; should use basic
	idmsMeta := GetMeta("kion_idms")
	got = updateTestValueWithMeta(attrInfo{name: "password_expiration", attrType: "int64"}, false, idmsMeta)
	if got != "0" {
		t.Errorf("updateTestValueWithMeta(password_expiration) = %q, want %q", got, "0")
	}
}

func TestBuildBasicConfig_WithDependencies(t *testing.T) {
	meta := GetMeta("kion_ou")
	attrs := []attrInfo{
		{name: "name", attrType: "string"},
		{name: "parent_ou_id", attrType: "int64"},
	}

	config := buildBasicConfig("kion_ou", attrs, true, meta)

	// Should contain the dependency resource
	if !strings.Contains(config, `resource "kion_permission_scheme" "test_perm"`) {
		t.Error("expected dependency resource kion_permission_scheme in config")
	}

	// Should contain a reference to the dependency
	if !strings.Contains(config, "permission_scheme_id = kion_permission_scheme.test_perm.id") {
		t.Error("expected permission_scheme_id reference in config")
	}

	// Should contain owner_users block
	if !strings.Contains(config, "owner_users { id = 1 }") {
		t.Error("expected owner_users block in config")
	}

	// parent_ou_id should use the override value
	if !strings.Contains(config, "parent_ou_id = 0") {
		t.Error("expected parent_ou_id = 0 from field override")
	}
}

func TestBuildBasicConfig_NoDependencies(t *testing.T) {
	meta := GetMeta("kion_label")
	attrs := []attrInfo{
		{name: "key", attrType: "string"},
		{name: "value", attrType: "string"},
		{name: "color", attrType: "string"},
	}

	config := buildBasicConfig("kion_label", attrs, false, meta)

	if !strings.Contains(config, `resource "kion_label" "test"`) {
		t.Error("expected resource block")
	}
	if !strings.Contains(config, `color = "#0088ff"`) {
		t.Error("expected color override in config")
	}
	// Should not contain dependency resources
	if strings.Contains(config, "kion_permission_scheme") {
		t.Error("did not expect dependency in label config")
	}
}
