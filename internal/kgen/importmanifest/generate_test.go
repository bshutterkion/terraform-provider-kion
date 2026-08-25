package importmanifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture() *Manifest {
	return Build(
		map[string]string{ // kind -> by-id read path, from generator_config.yaml resources:
			"ou":                        "/v3/ou/{id}",
			"aws_resource_tag":          "",
			"ou_enforcement":            "/v3/ou/{id}/enforcement",
			"ou_permission_mapping":     "/v3/ou/{id}/permission-mapping",
			"app_config":                "/v3/app-config",
			"global_permission_mapping": "/v3/global/permission-mapping",
			"account":                   "/v4/account-legacy/{id}", // must lose to the data source below
		},
		map[string]string{ // kind -> list read path, from generator_config.yaml data_sources:
			"account": "/v3/account",
		},
		map[string]archetypeInfo{ // kind -> archetype fields, from crud_archetypes.yaml
			"aws_resource_tag": {Kind: "no_read"},
			"ou_enforcement":   {Kind: "parent_list"},
			"ou_permission_mapping": {
				Kind: "association", KeyField: "app_role_id", ParentField: "ou_id",
			},
			"app_config": {Kind: "singleton"},
			"global_permission_mapping": {
				Kind: "association", KeyField: "app_role_id",
			},
		},
		[]string{
			"kion_ou", "kion_aws_resource_tag", "kion_ou_enforcement",
			"kion_ou_permission_mapping", "kion_app_config", "kion_account",
			"kion_global_permission_mapping",
		},
	)
}

func byType(m *Manifest, tfType string) Resource {
	for _, r := range m.Resources {
		if r.TFType == tfType {
			return r
		}
	}
	return Resource{}
}

func TestBuildEmitsARowPerTFType(t *testing.T) {
	t.Parallel()
	m := fixture()
	assert.Len(t, m.Resources, 7)
	assert.Equal(t, ManifestVersion, m.Version)
}

func TestBuildSortsByTFType(t *testing.T) {
	t.Parallel()
	m := fixture()
	for i := 1; i < len(m.Resources); i++ {
		assert.Less(t, m.Resources[i-1].TFType, m.Resources[i].TFType)
	}
}

func TestBuildGenericResource(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_ou")
	assert.Equal(t, ShapeGeneric, r.ReadShape)
	assert.Equal(t, "/v3/ou", r.ListPath)
	assert.Equal(t, FormatID, r.ImportID.Format)
	assert.True(t, r.Readable)
}

func TestBuildNoReadResourceCarriesReasonAndNoPath(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_aws_resource_tag")
	assert.False(t, r.Readable)
	assert.Empty(t, r.ListPath)
	assert.Contains(t, r.Reason, "no_read")
}

func TestBuildParentListResourceGetsItsParentBlock(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_ou_enforcement")
	require.NotNil(t, r.Parent)
	assert.Equal(t, "/v3/ou", r.Parent.ListPath)
	assert.Equal(t, "/v3/ou/{parent_id}/enforcement", r.Parent.ChildPath)
	assert.Equal(t, "ou_id", r.Parent.ParentIDField)
	assert.Equal(t, ShapeParentList, r.ReadShape)
	assert.Equal(t, FormatID, r.ImportID.Format)
	assert.True(t, r.Readable)
}

func TestBuildAssociationUsesParentSlashKey(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_ou_permission_mapping")
	assert.Equal(t, FormatParentSlashKey, r.ImportID.Format)
	assert.Equal(t, "app_role_id", r.ImportID.KeyField)
	require.NotNil(t, r.Parent)
	assert.Equal(t, "ou_id", r.Parent.ParentIDField)
	assert.Equal(t, ShapeAssociation, r.ReadShape)
}

func TestBuildSpecialResourceUsesItsBespokePath(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_app_config")
	assert.Equal(t, ShapeSpecial, r.ReadShape)
	assert.Equal(t, "/v3/app-config", r.ListPath)
}

// TestBuildDataSourcePathIsPreferredOverResourceReadPath guards the
// resolution order: generator_config.yaml's data_sources: entry is the real
// collection endpoint, and must win over the resources: by-id read even when
// both are present and disagree. kion_account's fixture inputs are set up so
// the two would produce different list paths if this rule were dropped
// (falling back to the by-id read would yield "/v4/account-legacy", not the
// data source's "/v3/account").
func TestBuildDataSourcePathIsPreferredOverResourceReadPath(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_account")
	assert.Equal(t, "/v3/account", r.ListPath)
	assert.True(t, r.Readable)
}

// TestBuildFallsBackToResourceReadPathWhenNoDataSource guards the other half
// of the resolution order: when generator_config.yaml has no data_sources:
// entry for a kind, the resources: by-id read is used instead of leaving the
// resource unreadable.
func TestBuildFallsBackToResourceReadPathWhenNoDataSource(t *testing.T) {
	t.Parallel()
	m := Build(
		map[string]string{"widget": "/v3/widget/{id}"},
		map[string]string{}, // no data source for "widget"
		map[string]archetypeInfo{},
		[]string{"kion_widget"},
	)
	r := byType(m, "kion_widget")
	assert.Equal(t, "/v3/widget", r.ListPath)
	assert.True(t, r.Readable)
}

// TestBuildAliasResolvesThroughKindAliases guards kindAliases: the provider
// serves kion_aws_cloudformation_template / kion_aws_iam_policy, but
// generator_config.yaml keys their generator_config entries "cft" /
// "iam_policy". Both readPaths and dataSourcePaths lookups must go through
// the alias, not the raw tf_type-derived kind.
func TestBuildAliasResolvesThroughKindAliases(t *testing.T) {
	t.Parallel()
	m := Build(
		map[string]string{
			"cft":        "/v3/cft/{id}",
			"iam_policy": "/v3/iam-policy/{id}",
		},
		map[string]string{
			"cft":        "/v3/cft",
			"iam_policy": "/v3/iam-policy",
		},
		map[string]archetypeInfo{},
		[]string{"kion_aws_cloudformation_template", "kion_aws_iam_policy"},
	)

	cft := byType(m, "kion_aws_cloudformation_template")
	assert.Equal(t, "/v3/cft", cft.ListPath)
	assert.True(t, cft.Readable)
	assert.Equal(t, "aws_cloudformation_template", cft.Kind, "Kind stays the tf_type-derived kind, not the alias")

	iamPolicy := byType(m, "kion_aws_iam_policy")
	assert.Equal(t, "/v3/iam-policy", iamPolicy.ListPath)
	assert.True(t, iamPolicy.Readable)
}

// TestBuildTwoPlaceholderPathIsUnreadable guards the compound-path veto: a
// chosen path with two or more "{...}" placeholders can't be enumerated from
// either id alone, so it must be marked unreadable even when its archetype
// (cv_override here) would otherwise fix it readable and ShapeSpecial. This
// is kion_custom_variable_override's
// /v3/account/{account_id}/custom-variable/{custom_variable_id}.
func TestBuildTwoPlaceholderPathIsUnreadable(t *testing.T) {
	t.Parallel()
	m := Build(
		map[string]string{
			"custom_variable_override": "/v3/account/{account_id}/custom-variable/{custom_variable_id}",
		},
		map[string]string{},
		map[string]archetypeInfo{
			"custom_variable_override": {Kind: "cv_override"},
		},
		[]string{"kion_custom_variable_override"},
	)
	r := byType(m, "kion_custom_variable_override")
	assert.False(t, r.Readable)
	assert.Equal(t, ShapeNone, r.ReadShape)
	assert.Empty(t, r.ListPath)
	assert.Nil(t, r.Parent)
	assert.Contains(t, r.Reason, "/v3/account/{account_id}/custom-variable/{custom_variable_id}")
}

// TestBuildParentlessAssociationUsesPlainID guards the else-branch of
// association.gtpl's ImportState: a parentless association (no parent_field
// declared, e.g. kion_global_permission_mapping) parses req.ID as a plain
// integer rather than splitting on "/". If Build claimed FormatParentSlashKey
// here, the downstream enumerator would emit zero import blocks, silently.
//
// That same else-branch assigns the parsed integer directly to the KEY
// field, not a synthesized id -- so for a parentless association the import
// id IS the key field's value, and KeyField must still be populated (Format
// stays FormatID) so the enumerator knows which raw field to read.
func TestBuildParentlessAssociationUsesPlainID(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_global_permission_mapping")
	assert.Equal(t, FormatID, r.ImportID.Format)
	assert.Equal(t, "app_role_id", r.ImportID.KeyField)
	assert.True(t, r.Readable)
	assert.Equal(t, "/v3/global/permission-mapping", r.ListPath)
	assert.Nil(t, r.Parent)
}

// TestBuildKeyFieldComesFromArchetypeData guards against re-hardcoding the
// association key: it must come from crud_archetypes.yaml's key_field, not a
// package constant. A key_field other than "app_role_id" must still surface
// verbatim.
func TestBuildKeyFieldComesFromArchetypeData(t *testing.T) {
	t.Parallel()
	m := Build(
		map[string]string{"widget_permission_mapping": "/v3/widget-permission-mapping/{id}"},
		map[string]string{},
		map[string]archetypeInfo{
			"widget_permission_mapping": {
				Kind: "association", KeyField: "widget_role_id", ParentField: "widget_id",
			},
		},
		[]string{"kion_widget_permission_mapping"},
	)
	r := byType(m, "kion_widget_permission_mapping")
	assert.Equal(t, FormatParentSlashKey, r.ImportID.Format)
	assert.Equal(t, "widget_role_id", r.ImportID.KeyField)
}

// TestBuildHasParentComesFromDeclaredParentField guards the split between the
// two concerns Build must not conflate: hasParent (the import-id FORMAT)
// comes from crud_archetypes.yaml's declared parent_field, mirroring how
// internal/kgen/crud/assoc.go derives association.gtpl's HasParent from
// arch.ParentField -- never from whether the chosen read path happens to
// carry a parent-scoping placeholder. KeyField, by contrast, is populated
// for an association either way -- with a parent it is half of
// "<parent>/<key>"; without one, per association.gtpl's {{else}} branch, it
// IS the whole import id.
func TestBuildHasParentComesFromDeclaredParentField(t *testing.T) {
	t.Parallel()
	readPaths := map[string]string{"ou_permission_mapping": "/v3/ou-permission-mapping/{id}"}
	tfTypes := []string{"kion_ou_permission_mapping"}

	withParentField := Build(readPaths, map[string]string{}, map[string]archetypeInfo{
		"ou_permission_mapping": {Kind: "association", KeyField: "app_role_id", ParentField: "ou_id"},
	}, tfTypes)
	r := byType(withParentField, "kion_ou_permission_mapping")
	assert.Equal(t, FormatParentSlashKey, r.ImportID.Format)
	assert.Equal(t, "app_role_id", r.ImportID.KeyField)

	withoutParentField := Build(readPaths, map[string]string{}, map[string]archetypeInfo{
		"ou_permission_mapping": {Kind: "association", KeyField: "app_role_id"},
	}, tfTypes)
	r2 := byType(withoutParentField, "kion_ou_permission_mapping")
	assert.Equal(t, FormatID, r2.ImportID.Format)
	assert.Equal(t, "app_role_id", r2.ImportID.KeyField)
}

// TestBuildParentScopedPathDerivesParentIDField guards the placeholder-based
// Parent derivation for a chosen path with exactly one "{...}" that is NOT
// the path's final segment -- a real nested collection, not a plain by-id
// GET. Each case is a real generator_config.yaml data_sources: entry with a
// different parent_id_field shape.
func TestBuildParentScopedPathDerivesParentIDField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		kind               string
		archetype          archetypeInfo
		path               string
		wantParentIDField  string
		wantParentListPath string
		wantChildPath      string
		wantShape          ReadShape
	}{
		{
			name: "ou_note", kind: "ou_note",
			path:               "/v3/ou/{id}/ou-note",
			wantParentIDField:  "ou_id",
			wantParentListPath: "/v3/ou",
			wantChildPath:      "/v3/ou/{parent_id}/ou-note",
			wantShape:          ShapeParentList,
		},
		{
			name: "project_enforcement", kind: "project_enforcement",
			archetype:          archetypeInfo{Kind: "parent_list"},
			path:               "/v3/project/{id}/enforcement",
			wantParentIDField:  "project_id",
			wantParentListPath: "/v3/project",
			wantChildPath:      "/v3/project/{parent_id}/enforcement",
			wantShape:          ShapeParentList,
		},
		{
			name: "idms_group_association", kind: "idms_group_association",
			path:               "/v3/idms/{id}/group-association",
			wantParentIDField:  "idms_id",
			wantParentListPath: "/v3/idms",
			wantChildPath:      "/v3/idms/{parent_id}/group-association",
			wantShape:          ShapeParentList,
		},
		{
			name: "compliance_control", kind: "compliance_control",
			archetype:          archetypeInfo{Kind: "entity"},
			path:               "/v4/compliance/program/{id}/control",
			wantParentIDField:  "compliance_program_id",
			wantParentListPath: "/v4/compliance/program",
			wantChildPath:      "/v4/compliance/program/{parent_id}/control",
			wantShape:          ShapeParentList,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			tfType := "kion_" + c.kind
			m := Build(
				map[string]string{},
				map[string]string{c.kind: c.path},
				map[string]archetypeInfo{c.kind: c.archetype},
				[]string{tfType},
			)
			r := byType(m, tfType)
			require.NotNil(t, r.Parent, c.name)
			assert.Equal(t, c.wantParentIDField, r.Parent.ParentIDField, c.name)
			assert.Equal(t, c.wantParentListPath, r.Parent.ListPath, c.name)
			assert.Equal(t, c.wantChildPath, r.Parent.ChildPath, c.name)
			assert.Equal(t, c.wantShape, r.ReadShape, c.name)
			assert.True(t, r.Readable, c.name)
		})
	}
}

// TestBuildOverridesOpenIDFamilyParent guards Fix 3: the OpenID family's
// derived parent list (the text before each resource's trailing "/{id}") is
// not itself listable -- /v4/idms/open-id 405s live. parentOverrides must
// replace each of the three resources' derivation with a Parent scoped by
// the IDMS id instead: ListPath "/v3/idms" (the real enumerable list),
// ParentIDField "idms_id", and the live-verified child path.
func TestBuildOverridesOpenIDFamilyParent(t *testing.T) {
	t.Parallel()
	m := Build(
		map[string]string{ // real generator_config.yaml resources: read paths
			"idms_open_id":                   "/v4/idms/open-id/{id}",
			"idms_open_id_access_rule":       "/v4/idms/open-id/access-rule/{id}",
			"idms_open_id_group_association": "/v4/idms/open-id/group-association/{id}",
		},
		map[string]string{},
		map[string]archetypeInfo{},
		[]string{
			"kion_idms_open_id",
			"kion_idms_open_id_access_rule",
			"kion_idms_open_id_group_association",
		},
	)

	cases := []struct {
		tfType        string
		wantChildPath string
	}{
		{"kion_idms_open_id", "/v4/idms/open-id/{parent_id}"},
		{"kion_idms_open_id_access_rule", "/v4/idms/open-id/{parent_id}/access-rule"},
		{"kion_idms_open_id_group_association", "/v4/idms/open-id/{parent_id}/group-association"},
	}
	for _, c := range cases {
		r := byType(m, c.tfType)
		require.NotNil(t, r.Parent, c.tfType)
		assert.Equal(t, "/v3/idms", r.Parent.ListPath, c.tfType)
		assert.Equal(t, "idms_id", r.Parent.ParentIDField, c.tfType)
		assert.Equal(t, "idms", r.Parent.Kind, c.tfType)
		assert.Equal(t, c.wantChildPath, r.Parent.ChildPath, c.tfType)
		assert.Equal(t, ShapeParentList, r.ReadShape, c.tfType)
		assert.True(t, r.Readable, c.tfType)
		assert.Empty(t, r.ListPath, c.tfType)
	}
}

func TestManifestMarshalsDeterministically(t *testing.T) {
	t.Parallel()
	a, err := json.MarshalIndent(fixture(), "", "  ")
	require.NoError(t, err)
	b, err := json.MarshalIndent(fixture(), "", "  ")
	require.NoError(t, err)
	assert.Equal(t, string(a), string(b))
}
