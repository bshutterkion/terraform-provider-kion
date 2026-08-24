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
