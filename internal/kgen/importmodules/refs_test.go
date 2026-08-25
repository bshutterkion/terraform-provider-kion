package importmodules_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"terraform-provider-kion/internal/kgen/importmodules"
)

// A reference to a block the rewrite converts becomes invalid the moment the
// resource stops existing under that name, and Rewrite copies expressions
// verbatim, so nothing else in the pipeline would notice.
func TestFindDanglingRefs_ReportsConvertedTarget(t *testing.T) {
	t.Parallel()
	src := `resource "kion_ou" "parent" {
  name = "eng"
}

resource "kion_ou" "child" {
  name         = "sub"
  parent_ou_id = kion_ou.parent.id
}
`
	got, err := importmodules.FindDanglingRefs([]byte(src), fixtureManifest())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "parent_ou_id", got[0].Attr)
	assert.Equal(t, "kion_ou.parent", got[0].Target)
	assert.Contains(t, got[0].From, "child")
}

// A reference to a type with NO manifest entry is fine: that block is left
// exactly where it was, so the reference still resolves.
func TestFindDanglingRefs_IgnoresUnrewrittenTarget(t *testing.T) {
	t.Parallel()
	src := `resource "not_kion_thing" "other" {
  a = 1
}

resource "kion_ou" "child" {
  name         = "sub"
  parent_ou_id = not_kion_thing.other.id
}
`
	got, err := importmodules.FindDanglingRefs([]byte(src), fixtureManifest())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFindDanglingRefs_NoneWhenNothingRewritten(t *testing.T) {
	t.Parallel()
	src := `resource "not_kion_thing" "a" {
  x = not_kion_thing.b.id
}
`
	got, err := importmodules.FindDanglingRefs([]byte(src), fixtureManifest())
	require.NoError(t, err)
	assert.Empty(t, got)
}

// var./local./data. traversals are not resource references and must not be
// reported, or every generated file with a variable in it would fail.
func TestFindDanglingRefs_VariablesAreNotReferences(t *testing.T) {
	t.Parallel()
	src := `variable "n" {}

resource "kion_ou" "child" {
  name = var.n
}
`
	got, err := importmodules.FindDanglingRefs([]byte(src), fixtureManifest())
	require.NoError(t, err)
	assert.Empty(t, got)
}
