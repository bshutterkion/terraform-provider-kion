package kimport

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"terraform-provider-kion/internal/kgen/importmanifest"
)

// routeLister serves canned records by exact path.
type routeLister struct {
	routes map[string]any // []map[string]any or error
	calls  []string
}

func (r *routeLister) List(_ context.Context, path string) ([]map[string]any, error) {
	r.calls = append(r.calls, path)
	v, ok := r.routes[path]
	if !ok {
		return nil, errors.New("404 " + path)
	}
	if err, isErr := v.(error); isErr {
		return nil, err
	}
	records, isRecords := v.([]map[string]any)
	if !isRecords {
		return nil, fmt.Errorf("routeLister: route %q is neither []map[string]any nor error", path)
	}
	return records, nil
}

func rec(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			panic(fmt.Sprintf("rec: key at index %d is not a string: %v", i, kv[i]))
		}
		m[key] = kv[i+1]
	}
	return m
}

func TestEnumerateGeneric(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/ou": []map[string]any{rec("id", float64(1), "name", "Root")},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou", ReadShape: importmanifest.ShapeGeneric,
		Readable: true, ListPath: "/v3/ou", NameField: "name",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	assert.Equal(t, "1", res.Records[0].ID)
	assert.Equal(t, "Root", res.Records[0].Name)
}

func TestEnumerateGenericEmptyIsNotAnError(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{"/v3/ou": []map[string]any{}}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou", ReadShape: importmanifest.ShapeGeneric,
		Readable: true, ListPath: "/v3/ou",
	})
	assert.Equal(t, "empty", res.Status)
}

func TestEnumerateGenericErrorIsReported(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v4/compliance/family": errors.New("GET: 405 Method Not Allowed"),
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_compliance_family", ReadShape: importmanifest.ShapeGeneric,
		Readable: true, ListPath: "/v4/compliance/family",
	})
	assert.Equal(t, "error", res.Status)
	assert.Contains(t, res.Reason, "405")
}

func TestEnumerateParentScoped(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/ou":               []map[string]any{rec("id", float64(1)), rec("id", float64(2))},
		"/v3/ou/1/enforcement": []map[string]any{rec("id", float64(10))},
		"/v3/ou/2/enforcement": []map[string]any{rec("id", float64(20))},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou_enforcement", ReadShape: importmanifest.ShapeParentList, Readable: true,
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/enforcement", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	assert.Len(t, res.Records, 2)
	assert.Equal(t, float64(1), res.Records[0].Raw["ou_id"])
}

func TestEnumerateParentScopedSurvivesOneBadParent(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/ou":               []map[string]any{rec("id", float64(1)), rec("id", float64(2))},
		"/v3/ou/1/enforcement": errors.New("500 boom"),
		"/v3/ou/2/enforcement": []map[string]any{rec("id", float64(20))},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou_enforcement", ReadShape: importmanifest.ShapeParentList, Readable: true,
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/enforcement", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	assert.Equal(t, "ok", res.Status)
	assert.Len(t, res.Records, 1)
	assert.Contains(t, res.Reason, "500")
}

func TestEnumerateAssociationBuildsParentSlashKey(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/ou":                      []map[string]any{rec("id", float64(3))},
		"/v3/ou/3/permission-mapping": []map[string]any{rec("app_role_id", float64(2))},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou_permission_mapping", ReadShape: importmanifest.ShapeAssociation, Readable: true,
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/permission-mapping", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatParentSlashKey, KeyField: "app_role_id"},
	})
	require.Equal(t, "ok", res.Status)
	assert.Equal(t, "3/2", res.Records[0].ID)
}

func TestEnumerateSpecial(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/app-config": []map[string]any{rec("smtp_host", "mail")},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_app_config", ReadShape: importmanifest.ShapeSpecial, Readable: true,
		ListPath: "/v3/app-config",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	assert.Equal(t, "app_config", res.Records[0].ID, "a singleton with no id uses its kind")
}

func TestEnumerateUnreadableIsUnsupportedNotAnError(t *testing.T) {
	t.Parallel()
	res := Enumerate(context.Background(), &routeLister{}, importmanifest.Resource{
		TFType: "kion_aws_resource_tag", ReadShape: importmanifest.ShapeNone,
		Readable: false, Reason: "kind: no_read",
	})
	assert.Equal(t, "unsupported", res.Status)
	assert.Equal(t, "kind: no_read", res.Reason)
	assert.Empty(t, res.Records)
}

// --- Correction 1: parent-scoped fallback when a generic flat list 405s. ---

func TestEnumerateGenericFallsBackToParentScopedOnFlatListFailure(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v4/compliance/family":      errors.New("GET: 405 Method Not Allowed"),
		"/v3/ou":                     []map[string]any{rec("id", float64(1))},
		"/v3/ou/1/compliance/family": []map[string]any{rec("id", float64(10))},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_compliance_family", ReadShape: importmanifest.ShapeGeneric, Readable: true,
		ListPath: "/v4/compliance/family",
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/compliance/family", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	require.Len(t, res.Records, 1)
	assert.Equal(t, "10", res.Records[0].ID)
	assert.Contains(t, res.Reason, "flat list failed")
	assert.Contains(t, res.Reason, "405")
	assert.Contains(t, res.Reason, "parent-scoped")
}

func TestEnumerateGenericFallsBackAndCanBeEmpty(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v4/compliance/level":      errors.New("GET: 405 Method Not Allowed"),
		"/v3/ou":                    []map[string]any{rec("id", float64(1))},
		"/v3/ou/1/compliance/level": []map[string]any{},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_compliance_level", ReadShape: importmanifest.ShapeGeneric, Readable: true,
		ListPath: "/v4/compliance/level",
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/compliance/level", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	assert.Equal(t, "empty", res.Status)
	assert.Empty(t, res.Records)
	assert.Contains(t, res.Reason, "flat list failed")
}

func TestEnumerateGenericNoParentFallbackIsAnError(t *testing.T) {
	t.Parallel()
	// Same as TestEnumerateGenericErrorIsReported, named to make the (b) case
	// from the flat-list-fails-with-no-Parent correction explicit.
	l := &routeLister{routes: map[string]any{
		"/v4/compliance/family": errors.New("GET: 405 Method Not Allowed"),
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_compliance_family", ReadShape: importmanifest.ShapeGeneric, Readable: true,
		ListPath: "/v4/compliance/family",
		// Parent intentionally omitted.
	})
	assert.Equal(t, "error", res.Status)
	assert.Contains(t, res.Reason, "405")
}

func TestEnumerateGenericFallbackAlsoFailsReportsOriginalError(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v4/compliance/family": errors.New("GET: 405 Method Not Allowed"),
		"/v3/ou":                errors.New("500 boom"),
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_compliance_family", ReadShape: importmanifest.ShapeGeneric, Readable: true,
		ListPath: "/v4/compliance/family",
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/compliance/family", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	assert.Equal(t, "error", res.Status)
	assert.Contains(t, res.Reason, "405")
	assert.NotContains(t, res.Reason, "500")
}

// --- Correction 2: the singleton id fallback is ShapeSpecial-only. ---

func TestEnumerateSpecialSingletonWithNoIDUsesKind(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/app-config": []map[string]any{rec("smtp_host", "mail")},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_app_config", ReadShape: importmanifest.ShapeSpecial, Readable: true,
		ListPath: "/v3/app-config",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	require.Len(t, res.Records, 1)
	assert.Equal(t, "app_config", res.Records[0].ID)
}

func TestEnumerateGenericSkipsRecordsWithNoID(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/global-permission-mapping": []map[string]any{
			rec("app_role_id", float64(1)),
			rec("app_role_id", float64(2)),
		},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_global_permission_mapping", ReadShape: importmanifest.ShapeGeneric, Readable: true,
		ListPath: "/v3/global-permission-mapping",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	assert.Equal(t, "empty", res.Status)
	assert.Empty(t, res.Records)
	assert.Contains(t, res.Reason, "2 record(s) skipped: no id")
}

// --- Correction 3: association records missing the key field are skipped
// and counted, not silently dropped. ---

// --- Bug B: per-type wrapper shape ({"cft":{...,"id":296},"owner_users":[...],
// "owner_user_groups":[...]}) is unwrapped before id/name extraction. ---

func TestToRecordsUnwrapsPerTypeWrapper(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		raw    map[string]any
		wantID string
	}{
		{
			name:   "cft",
			raw:    rec("cft", rec("id", float64(296)), "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "296",
		},
		{
			name:   "ami",
			raw:    rec("ami", rec("id", float64(2)), "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "2",
		},
		{
			name:   "iam_policy",
			raw:    rec("iam_policy", rec("id", float64(3853)), "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "3853",
		},
		{
			// The trap: the wrapper key ("service_catalog_portfolio") does not
			// match the resource's tf kind ("service_catalog"), so detection
			// must be structural, not derived from TFType.
			name:   "service_catalog wrapped under a differently-named key",
			raw:    rec("service_catalog_portfolio", rec("id", float64(4)), "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "4",
		},
		{
			name:   "wrapper with no owner arrays at all",
			raw:    rec("ami", rec("id", float64(7))),
			wantID: "7",
		},
		{
			// Real shape observed live: /v3/cft -> ['cft', 'owner_user_groups', 'owner_users', 'tags'].
			name: "cft with tags sibling",
			raw: rec("cft", rec("id", float64(296)), "owner_users", []any{}, "owner_user_groups", []any{},
				"tags", []any{}),
			wantID: "296",
		},
		{
			// Real shape observed live: /v3/azure-policy -> ['azure_policy',
			// 'compliance_programs', 'owner_user_groups', 'owner_users'].
			name: "azure_policy with compliance_programs sibling",
			raw: rec("azure_policy", rec("id", float64(11)), "compliance_programs", []any{},
				"owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "11",
		},
		{
			// Real shape observed live: /v3/azure-role -> ['azure_role',
			// 'car_restricted_ugroups', 'car_restricted_users', 'owner_user_groups', 'owner_users'].
			name: "azure_role with car_restricted siblings",
			raw: rec("azure_role", rec("id", float64(22)), "car_restricted_ugroups", []any{},
				"car_restricted_users", []any{}, "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "22",
		},
		{
			// Real shape observed live: /v3/gcp-iam-role -> ['car_restricted_ugroups',
			// 'car_restricted_users', 'gcp_role', 'owner_user_groups', 'owner_users'].
			// The trap: wrapped under "gcp_role", not the resource kind "gcp_iam_role".
			name: "gcp_iam_role wrapped under gcp_role",
			raw: rec("gcp_role", rec("id", float64(33)), "car_restricted_ugroups", []any{},
				"car_restricted_users", []any{}, "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "33",
		},
		{
			// Real shape observed live: /v3/azure-arm-template -> ['azure_arm_template',
			// 'compliance_programs', 'is_enabled', 'owner_user_groups', 'owner_users'].
			// "is_enabled" is a scalar sibling, not a map -- must not be mistaken for a
			// second candidate.
			name: "azure_arm_template with compliance_programs and scalar is_enabled siblings",
			raw: rec("azure_arm_template", rec("id", float64(44)), "compliance_programs", []any{},
				"is_enabled", true, "owner_users", []any{}, "owner_user_groups", []any{}),
			wantID: "44",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			records, skipped := toRecords([]map[string]any{tt.raw}, importmanifest.Resource{
				TFType: "kion_x", ReadShape: importmanifest.ShapeGeneric,
				ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
			}, "")
			require.Equal(t, 0, skipped)
			require.Len(t, records, 1)
			assert.Equal(t, tt.wantID, records[0].ID)
			// The outer wrapper object is preserved as Raw, not the unwrapped inner one.
			assert.Equal(t, tt.raw, records[0].Raw)
		})
	}
}

func TestToRecordsExtractsNameFromWrapper(t *testing.T) {
	t.Parallel()
	raw := rec("cft", rec("id", float64(296), "name", "MyCFT"), "owner_users", []any{})
	records, skipped := toRecords([]map[string]any{raw}, importmanifest.Resource{
		TFType: "kion_cft", ReadShape: importmanifest.ShapeGeneric, NameField: "name",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	}, "")
	require.Equal(t, 0, skipped)
	require.Len(t, records, 1)
	assert.Equal(t, "296", records[0].ID)
	assert.Equal(t, "MyCFT", records[0].Name)
}

// TestToRecordsPlainRecordUnchanged is a regression guard: a normal,
// already-flat record must extract exactly as before.
func TestToRecordsPlainRecordUnchanged(t *testing.T) {
	t.Parallel()
	raw := rec("id", float64(1), "name", "x")
	records, skipped := toRecords([]map[string]any{raw}, importmanifest.Resource{
		TFType: "kion_x", ReadShape: importmanifest.ShapeGeneric, NameField: "name",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	}, "")
	require.Equal(t, 0, skipped)
	require.Len(t, records, 1)
	assert.Equal(t, "1", records[0].ID)
	assert.Equal(t, "x", records[0].Name)
}

// TestToRecordsAmbiguousTwoNonOwnerKeysNotUnwrapped: two non-owner keys is too
// ambiguous to guess a wrapper key, so the record is left as-is -- which means
// it has no top-level "id" and is skipped, same as any other id-less record.
func TestToRecordsAmbiguousTwoNonOwnerKeysNotUnwrapped(t *testing.T) {
	t.Parallel()
	raw := rec("cft", rec("id", float64(1)), "other_key", rec("id", float64(2)))
	records, skipped := toRecords([]map[string]any{raw}, importmanifest.Resource{
		TFType: "kion_x", ReadShape: importmanifest.ShapeGeneric,
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	}, "")
	assert.Equal(t, 1, skipped)
	assert.Empty(t, records)
}

// TestToRecordsSiblingWithoutIDDoesNotBlockDetection: a sibling map-valued key
// that lacks an "id" (e.g. a "meta" blob) does not count as a second
// candidate -- only id-bearing maps compete, so the one qualifying key
// ("thing") is still detected.
func TestToRecordsSiblingWithoutIDDoesNotBlockDetection(t *testing.T) {
	t.Parallel()
	raw := rec(
		"thing", rec("id", float64(1), "name", "T"),
		"meta", rec("note", "x"),
	)
	records, skipped := toRecords([]map[string]any{raw}, importmanifest.Resource{
		TFType: "kion_x", ReadShape: importmanifest.ShapeGeneric, NameField: "name",
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	}, "")
	require.Equal(t, 0, skipped)
	require.Len(t, records, 1)
	assert.Equal(t, "1", records[0].ID)
	assert.Equal(t, "T", records[0].Name)
}

// --- I1: parentScopedResult must run each parent through unwrapTypedRecord
// before reading its id. If a parent list ever uses the per-type wrapper
// shape (the exact shape unwrapTypedRecord exists for), extracting
// parent["id"] directly finds nothing, every parent is silently `continue`d,
// and the resource reports "empty, 0 records, reason \"\"" -- no count, no
// reason, nowhere. ---

func TestEnumerateParentScopedUnwrapsWrappedParentList(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		// The parent list itself uses the per-type wrapper shape, e.g. what
		// /v3/ou would look like if it ever wrapped under "ou" the way
		// /v3/cft wraps under "cft".
		"/v3/ou":               []map[string]any{rec("ou", rec("id", float64(1)))},
		"/v3/ou/1/enforcement": []map[string]any{rec("id", float64(10))},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou_enforcement", ReadShape: importmanifest.ShapeParentList, Readable: true,
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/enforcement", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	require.Len(t, res.Records, 1)
	assert.Equal(t, "10", res.Records[0].ID)
	assert.Empty(t, res.Reason)
}

func TestEnumerateParentScopedCountsParentsMissingID(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		// One parent has a usable id, one genuinely lacks one (no top-level
		// id and no single unwrappable candidate).
		"/v3/ou":               []map[string]any{rec("id", float64(1)), rec("name", "no id here")},
		"/v3/ou/1/enforcement": []map[string]any{rec("id", float64(10))},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou_enforcement", ReadShape: importmanifest.ShapeParentList, Readable: true,
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/enforcement", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatID},
	})
	require.Equal(t, "ok", res.Status)
	require.Len(t, res.Records, 1)
	assert.Contains(t, res.Reason, "1 parent(s) skipped: no id")
}

func TestEnumerateAssociationSkipsRecordsMissingKeyField(t *testing.T) {
	t.Parallel()
	l := &routeLister{routes: map[string]any{
		"/v3/ou": []map[string]any{rec("id", float64(3))},
		"/v3/ou/3/permission-mapping": []map[string]any{
			rec("app_role_id", float64(2)),
			rec("something_else", "x"), // no app_role_id: unresolvable key
		},
	}}
	res := Enumerate(context.Background(), l, importmanifest.Resource{
		TFType: "kion_ou_permission_mapping", ReadShape: importmanifest.ShapeAssociation, Readable: true,
		Parent: &importmanifest.Parent{
			Kind: "ou", ListPath: "/v3/ou",
			ChildPath: "/v3/ou/{parent_id}/permission-mapping", ParentIDField: "ou_id",
		},
		ImportID: importmanifest.ImportID{Format: importmanifest.FormatParentSlashKey, KeyField: "app_role_id"},
	})
	require.Equal(t, "ok", res.Status)
	require.Len(t, res.Records, 1)
	assert.Equal(t, "3/2", res.Records[0].ID)
	assert.Contains(t, res.Reason, "1 record(s) skipped: no id")
}
