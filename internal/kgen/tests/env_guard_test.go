package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSDKImportPathMatchesProvider pins the sub-package kgen writes into
// generated tests to the one internal/conns builds its *generated.Client from.
// A generated test passes generated.<Op>Params to conn.Client.<Op>, so a
// mismatch is a compile error in every emitted test that has an SDKGetMethod.
// This drifted once: the provider moved to v3_16 while this stayed on v3_15.
func TestSDKImportPathMatchesProvider(t *testing.T) {
	t.Parallel()

	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("finding project root: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, "internal", "conns", "kion_client.go"))
	if err != nil {
		t.Fatalf("reading kion_client.go: %v", err)
	}
	want := `generated "` + SDKImportPath + `"`
	if !strings.Contains(string(src), want) {
		t.Errorf("SDKImportPath is %q, which internal/conns/kion_client.go does not import;\n"+
			"generated tests would not compile against conn.Client", SDKImportPath)
	}
}

// envGuardMeta exercises RequiredEnv and KnownIssues together.
func envGuardMeta() *ResourceMeta {
	return &ResourceMeta{
		TypeName: "kion_thing",
		RequiredEnv: []EnvRequirement{
			{Name: "KION_ACC_BILLING_SOURCE_ID", Reason: "the ID of an existing billing source"},
		},
		KnownIssues: []string{"#99 thing drops its owner on read"},
	}
}

func TestWriteEnvSkips(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	writeEnvSkips(&b, envGuardMeta())
	out := b.String()

	for _, want := range []string{
		`os.Getenv("KION_ACC_BILLING_SOURCE_ID") == ""`,
		`t.Skip("KION_ACC_BILLING_SOURCE_ID must be set to the ID of an existing billing source")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("env skip guard missing %q, got:\n%s", want, out)
		}
	}
}

func TestWriteEnvSkips_NoRequirements(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	writeEnvSkips(&b, &ResourceMeta{TypeName: "kion_thing"})
	if b.Len() != 0 {
		t.Errorf("expected no guard for a resource with no RequiredEnv, got %q", b.String())
	}
}

func TestWriteKnownIssues(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	writeKnownIssues(&b, envGuardMeta())
	if want := "// Known issues this test is expected to surface:"; !strings.Contains(b.String(), want) {
		t.Errorf("known-issues header missing %q, got:\n%s", want, b.String())
	}
	if want := "//   #99 thing drops its owner on read"; !strings.Contains(b.String(), want) {
		t.Errorf("known-issues body missing %q, got:\n%s", want, b.String())
	}
}

// TestBuildBasicConfig_RequiredDependencyIsReferenced covers the case that made
// kion_compliance_family unrunnable: its dependency's target field
// (compliance_program_id) is Required, so the config filled it from the generic
// int fallback and referenced the program it had just created nowhere. Create
// then posted a program id of 1 and the API answered 404.
func TestBuildBasicConfig_RequiredDependencyIsReferenced(t *testing.T) {
	t.Parallel()

	meta := &ResourceMeta{
		TypeName: "kion_child",
		Dependencies: []Dependency{{
			TypeName:     "kion_parent",
			RefName:      "test_parent",
			Fields:       map[string]string{"name": `"test-acc-parent"`},
			RefAttribute: "id",
			TargetField:  "parent_id",
		}},
	}
	attrs := []attrInfo{
		{name: "name", attrType: "string"},
		{name: "parent_id", attrType: "int64"},
	}

	out := buildBasicConfig("kion_child", attrs, true, meta)

	if want := "parent_id = kion_parent.test_parent.id"; !strings.Contains(out, want) {
		t.Errorf("config missing %q, got:\n%s", want, out)
	}
	if strings.Contains(out, "parent_id = 1") {
		t.Errorf("config still hard-codes the dependency id, got:\n%s", out)
	}
}

func TestBuildUpdateConfig_RequiredDependencyIsReferenced(t *testing.T) {
	t.Parallel()

	meta := &ResourceMeta{
		TypeName: "kion_child",
		Dependencies: []Dependency{{
			TypeName:     "kion_parent",
			RefName:      "test_parent",
			Fields:       map[string]string{"name": `"test-acc-parent"`},
			RefAttribute: "id",
			TargetField:  "parent_id",
		}},
	}
	attrs := []attrInfo{{name: "parent_id", attrType: "int64"}}

	out := buildUpdateConfig("kion_child", attrs, labelLikeSchema(), true, meta)

	if want := "parent_id = kion_parent.test_parent.id"; !strings.Contains(out, want) {
		t.Errorf("update config missing %q, got:\n%s", want, out)
	}
	// The generic int update value would silently repoint the child at a
	// different, probably nonexistent, parent.
	if strings.Contains(out, "parent_id = 2") {
		t.Errorf("update config repoints the dependency by literal, got:\n%s", out)
	}
}

// TestBuildResourceTestFile_NoUpdate checks that a resource with no update
// endpoint gets no _update test, and no orphaned config function.
func TestBuildResourceTestFile_NoUpdate(t *testing.T) {
	// Not parallel: mutates the package-level registry.
	registry["kion_frozen"] = ResourceMeta{TypeName: "kion_frozen", NoUpdate: true}
	t.Cleanup(func() { delete(registry, "kion_frozen") })

	out := buildResourceTestFile("frozen", "kion_frozen", "frozen", "Frozen", labelLikeSchema())

	for _, unwanted := range []string{
		"func TestAccKionFrozen_update(",
		"func testAccFrozenConfig_update(",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("NoUpdate resource still got %q", unwanted)
		}
	}
	if want := "func TestAccKionFrozen_basic("; !strings.Contains(out, want) {
		t.Errorf("generated file missing %q", want)
	}
}

// TestBuildResourceTestFile_EnvGuarded checks the whole file wires os.Getenv in:
// the import, the guard in each test function, and the issue header.
func TestBuildResourceTestFile_EnvGuarded(t *testing.T) {
	// Not parallel: mutates the package-level registry.
	registry["kion_thing"] = *envGuardMeta()
	t.Cleanup(func() { delete(registry, "kion_thing") })

	out := buildResourceTestFile("thing", "kion_thing", "thing", "Thing", labelLikeSchema())

	for _, want := range []string{
		"// Known issues this test is expected to surface:",
		"\t\"os\"\n",
		`t.Skip("KION_ACC_BILLING_SOURCE_ID must be set to the ID of an existing billing source")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated test file missing %q", want)
		}
	}
	// Both _basic and _update carry the guard.
	if got := strings.Count(out, `os.Getenv("KION_ACC_BILLING_SOURCE_ID")`); got != 2 {
		t.Errorf("expected the env guard in both test funcs, got %d occurrences", got)
	}
}
