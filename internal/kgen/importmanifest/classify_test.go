// internal/kgen/importmanifest/classify_test.go
package importmanifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPathFrom(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"/v3/ou/{id}", "/v3/ou"},
		{"/v4/billing-source", "/v4/billing-source"},
		{"/v3/account-cache/{id}", "/v3/account-cache"},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ListPathFrom(c.in), c.in)
	}
}

func TestClassifyEntityIsGenericWithPlainID(t *testing.T) {
	t.Parallel()
	shape, format, readable, reason := Classify("entity", "/v3/ou/{id}", false)
	assert.Equal(t, ShapeGeneric, shape)
	assert.Equal(t, FormatID, format)
	assert.True(t, readable)
	assert.Empty(t, reason)
}

func TestClassifyArchetypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		archetype string
		shape     ReadShape
		format    IDFormat
		readable  bool
		hasParent bool
	}{
		{"entity", ShapeGeneric, FormatID, true, false},
		{"blended", ShapeGeneric, FormatID, true, false},
		{"cloud_account", ShapeGeneric, FormatID, true, false},
		{"parent_list", ShapeParentList, FormatID, true, false},
		{"compound_key_parent_read", ShapeParentList, FormatID, true, false},
		{"association", ShapeAssociation, FormatParentSlashKey, true, true},
		{"singleton", ShapeSpecial, FormatID, true, false},
		{"raw_http", ShapeSpecial, FormatID, true, false},
		{"cv_override", ShapeSpecial, FormatID, true, false},
		{"no_read", ShapeNone, FormatID, false, false},
		{"datasource_only", ShapeNone, FormatID, false, false},
	}
	for _, c := range cases {
		shape, format, readable, _ := Classify(c.archetype, "/v3/x/{id}", c.hasParent)
		assert.Equal(t, c.shape, shape, c.archetype)
		assert.Equal(t, c.format, format, c.archetype)
		assert.Equal(t, c.readable, readable, c.archetype)
	}
}

func TestClassifyNoReadCarriesAReason(t *testing.T) {
	t.Parallel()
	_, _, readable, reason := Classify("no_read", "", false)
	assert.False(t, readable)
	assert.Contains(t, reason, "no_read")
}

func TestClassifyMissingReadPathIsUnreadable(t *testing.T) {
	t.Parallel()
	shape, _, readable, reason := Classify("entity", "", false)
	assert.Equal(t, ShapeNone, shape)
	assert.False(t, readable)
	assert.Contains(t, reason, "no read path")
}

// TestClassifyParentlessAssociationImportsByPlainID verifies that associations
// without a parent (e.g., global_permission_mapping) import by plain integer id,
// not parent/key pairs. See association.gtpl's {{else}} branch.
func TestClassifyParentlessAssociationImportsByPlainID(t *testing.T) {
	t.Parallel()
	shape, format, readable, reason := Classify("association", "/v3/global-permission-mapping", false)
	assert.Equal(t, ShapeAssociation, shape)
	assert.Equal(t, FormatID, format)
	assert.True(t, readable)
	assert.Empty(t, reason)
}
