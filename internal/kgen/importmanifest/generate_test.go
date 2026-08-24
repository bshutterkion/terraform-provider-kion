package importmanifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture() *Manifest {
	return Build(
		map[string]string{ // kind -> read path, from generator_config.yaml
			"ou":                        "/v3/ou/{id}",
			"aws_resource_tag":          "",
			"ou_enforcement":            "/v3/ou-enforcement/{id}",
			"ou_permission_mapping":     "/v3/ou-permission-mapping/{id}",
			"app_config":                "/v3/app-config",
			"global_permission_mapping": "",
		},
		map[string]string{ // kind -> archetype, from crud_archetypes.yaml
			"aws_resource_tag":          "no_read",
			"ou_enforcement":            "parent_list",
			"ou_permission_mapping":     "association",
			"app_config":                "singleton",
			"global_permission_mapping": "association",
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
	assert.Contains(t, r.Parent.ChildPath, "{parent_id}")
	assert.Equal(t, FormatID, r.ImportID.Format)
}

func TestBuildAssociationUsesParentSlashKey(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_ou_permission_mapping")
	assert.Equal(t, FormatParentSlashKey, r.ImportID.Format)
	assert.Equal(t, "app_role_id", r.ImportID.KeyField)
	require.NotNil(t, r.Parent)
}

func TestBuildSpecialResourceUsesItsBespokePath(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_app_config")
	assert.Equal(t, ShapeSpecial, r.ReadShape)
	assert.Equal(t, "/v3/app-config", r.ListPath)
}

func TestBuildUsesExtraListPathsForCodegenGaps(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_account")
	assert.Equal(t, "/v3/account", r.ListPath)
	assert.True(t, r.Readable)
}

// TestBuildParentlessAssociationUsesPlainID guards the else-branch of
// association.gtpl's ImportState: a parentless association (no entry in
// ParentPaths, only in GlobalAssociationPaths) parses req.ID as a plain
// integer rather than splitting on "/". If Build claimed
// FormatParentSlashKey here, the downstream enumerator would emit zero
// import blocks for kion_global_permission_mapping, silently.
func TestBuildParentlessAssociationUsesPlainID(t *testing.T) {
	t.Parallel()
	r := byType(fixture(), "kion_global_permission_mapping")
	assert.Equal(t, FormatID, r.ImportID.Format)
	assert.Empty(t, r.ImportID.KeyField)
	assert.True(t, r.Readable)
}

func TestManifestMarshalsDeterministically(t *testing.T) {
	t.Parallel()
	a, err := json.MarshalIndent(fixture(), "", "  ")
	require.NoError(t, err)
	b, err := json.MarshalIndent(fixture(), "", "  ")
	require.NoError(t, err)
	assert.Equal(t, string(a), string(b))
}
