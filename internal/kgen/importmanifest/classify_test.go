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
	shape, format, readable, reason := Classify("ou", "entity", "/v3/ou/{id}")
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
	}{
		{"entity", ShapeGeneric, FormatID, true},
		{"blended", ShapeGeneric, FormatID, true},
		{"cloud_account", ShapeGeneric, FormatID, true},
		{"parent_list", ShapeParentList, FormatID, true},
		{"compound_key_parent_read", ShapeParentList, FormatID, true},
		{"association", ShapeAssociation, FormatParentSlashKey, true},
		{"singleton", ShapeSpecial, FormatID, true},
		{"raw_http", ShapeSpecial, FormatID, true},
		{"cv_override", ShapeSpecial, FormatID, true},
		{"no_read", ShapeNone, FormatID, false},
		{"datasource_only", ShapeNone, FormatID, false},
	}
	for _, c := range cases {
		shape, format, readable, _ := Classify("x", c.archetype, "/v3/x/{id}")
		assert.Equal(t, c.shape, shape, c.archetype)
		assert.Equal(t, c.format, format, c.archetype)
		assert.Equal(t, c.readable, readable, c.archetype)
	}
}

func TestClassifyNoReadCarriesAReason(t *testing.T) {
	t.Parallel()
	_, _, readable, reason := Classify("aws_resource_tag", "no_read", "")
	assert.False(t, readable)
	assert.Contains(t, reason, "no_read")
}

func TestClassifyMissingReadPathIsUnreadable(t *testing.T) {
	t.Parallel()
	shape, _, readable, reason := Classify("mystery", "entity", "")
	assert.Equal(t, ShapeNone, shape)
	assert.False(t, readable)
	assert.Contains(t, reason, "no read path")
}
