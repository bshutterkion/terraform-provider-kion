package rewriteref_test

import (
	"strings"
	"testing"

	"terraform-provider-kion/codegen"
	"terraform-provider-kion/internal/kgen/references"
	"terraform-provider-kion/internal/kimport/rewriteref"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func refs(t *testing.T) *references.Map {
	t.Helper()
	m, err := references.Parse(codegen.ReferencesYAML)
	require.NoError(t, err)
	return m
}

// realImports is the shape kion-import writes, including a compound
// "<parent>/<id>" id for a parent-scoped resource.
const realImports = `
import {
  to = kion_ou.kion_ou_11
  id = "11"
}
import {
  to = kion_ou.kion_ou_1
  id = "1"
}
import {
  to = kion_project.kion_project_9
  id = "9"
}
import {
  to = kion_user.kion_user_4
  id = "4"
}
import {
  to = kion_user.kion_user_94
  id = "94"
}
import {
  to = kion_ou_cloud_access_role_exemption.ex_1_4
  id = "1/4"
}
`

func index(t *testing.T) rewriteref.Index {
	t.Helper()
	idx, err := rewriteref.BuildIndex([]byte(realImports), "imports.tf")
	require.NoError(t, err)
	return idx
}

// TestBuildIndexFromImportBlocks: generated.tf has no ids in it -- they are
// Computed, so -generate-config-out omits them -- so the id -> label index can
// only come from the import blocks.
func TestBuildIndexFromImportBlocks(t *testing.T) {
	idx := index(t)

	assert.Equal(t, "kion_ou_11", idx["kion_ou"]["11"])
	assert.Equal(t, "kion_project_9", idx["kion_project"]["9"])
	assert.Equal(t, "kion_user_94", idx["kion_user"]["94"])

	// A compound import id is indexed by its record segment too, because a
	// foreign key holds the plain record id, not "<parent>/<id>".
	ex := idx["kion_ou_cloud_access_role_exemption"]
	assert.Equal(t, "ex_1_4", ex["4"], "record id resolves")
	assert.Equal(t, "ex_1_4", ex["1/4"], "full compound id resolves too")
}

// TestRewriteScalarAndList is the core case: a single FK and a list of them.
func TestRewriteScalarAndList(t *testing.T) {
	in := `
resource "kion_project" "kion_project_9" {
  name           = "Web"
  ou_id          = 11
  owner_user_ids = [4, 94]
}
`
	out, res, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)
	got := string(out)

	assert.Contains(t, got, "ou_id          = kion_ou.kion_ou_11.id")
	assert.Contains(t, got, "owner_user_ids = [kion_user.kion_user_4.id, kion_user.kion_user_94.id]")
	assert.Contains(t, got, `name           = "Web"`, "non-reference attributes untouched")
	assert.Equal(t, 3, res.Rewritten)
	assert.Empty(t, res.Unresolved)
}

// TestNotAReferenceIsLeftAlone covers the trap the authored table exists for:
// portfolio_id is an AWS id sitting beside internal_portfolio_ids, which IS a
// kion_service_catalog. Rewriting the former would point at a foreign id space.
func TestNotAReferenceIsLeftAlone(t *testing.T) {
	in := `
resource "kion_service_catalog" "sc" {
  portfolio_id    = 11
  account_type_id = 1
  car_external_id = 9
}
`
	out, res, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	got := string(out)
	assert.Contains(t, got, "portfolio_id    = 11", "an AWS id is not a Kion reference")
	assert.Contains(t, got, "account_type_id = 1", "an enum is not a reference")
	assert.Contains(t, got, "car_external_id = 9")
	assert.Zero(t, res.Rewritten)
	assert.Empty(t, res.Unresolved, "a non-reference is not a boundary gap either")
}

// TestUnresolvedIsReportedNotRewritten: a referent that is not managed in this
// configuration is the boundary. The literal stays, and it is reported so the
// operator knows what needs a data source, a variable, or another import.
func TestUnresolvedIsReportedNotRewritten(t *testing.T) {
	in := `
resource "kion_project" "kion_project_9" {
  ou_id          = 11
  owner_user_ids = [4, 777]
}
`
	out, res, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	got := string(out)
	assert.Contains(t, got, "kion_user.kion_user_4.id", "the resolvable one is rewritten")
	assert.Contains(t, got, "777", "the unresolvable one is left as a literal")

	require.Len(t, res.Unresolved, 1)
	assert.Equal(t, "kion_project.kion_project_9", res.Unresolved[0].Resource)
	assert.Equal(t, "owner_user_ids", res.Unresolved[0].Attr)
	assert.Equal(t, "kion_user", res.Unresolved[0].Target)
	assert.Equal(t, "777", res.Unresolved[0].Value)
	assert.Equal(t, map[string]int{"kion_user": 1}, res.TargetCounts())
}

// TestParentOUZeroKeepsSentinel: parent_ou_id = 0 means "top-level OU". A
// reference there would invent a parent that does not exist.
func TestParentOUZeroKeepsSentinel(t *testing.T) {
	in := `
resource "kion_ou" "kion_ou_1" {
  name         = "Root"
  parent_ou_id = 0
}
`
	out, res, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	assert.Contains(t, string(out), "parent_ou_id = 0")
	assert.Zero(t, res.Rewritten)
	assert.Empty(t, res.Unresolved, "0 is a sentinel, not a missing referent")
}

// TestSelfReferenceIsRefused: a block whose FK resolves to itself would be a
// cycle Terraform refuses to plan.
func TestSelfReferenceIsRefused(t *testing.T) {
	in := `
resource "kion_ou" "kion_ou_11" {
  parent_ou_id = 11
}
`
	out, res, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	assert.Contains(t, string(out), "parent_ou_id = 11")
	assert.Zero(t, res.Rewritten)
}

// TestNestedOUHierarchyBecomesAGraph is the payoff: once parents are
// references, Terraform orders creation itself. This is what makes the same
// configuration apply to a different install with no id map.
func TestNestedOUHierarchyBecomesAGraph(t *testing.T) {
	in := `
resource "kion_ou" "kion_ou_11" {
  name         = "Engineering"
  parent_ou_id = 1
}
`
	out, res, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	assert.Contains(t, string(out), "parent_ou_id = kion_ou.kion_ou_1.id")
	assert.Equal(t, 1, res.Rewritten)
}

// TestAlreadyReferencedIsIdempotent: running twice must not corrupt a
// configuration that already holds references.
func TestAlreadyReferencedIsIdempotent(t *testing.T) {
	in := `
resource "kion_project" "kion_project_9" {
  ou_id = 11
}
`
	once, _, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)
	twice, res, err := rewriteref.Rewrite(once, "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	assert.Equal(t, string(once), string(twice))
	assert.Zero(t, res.Rewritten, "nothing left to rewrite")
	assert.Empty(t, res.Unresolved)
}

// TestCommentsAndFormattingSurvive: -generate-config-out writes a provenance
// comment above every block, and losing it would make the output harder to
// review than what it replaced.
func TestCommentsAndFormattingSurvive(t *testing.T) {
	in := `# __generated__ by Terraform
# __generated__ by Terraform from "9"
resource "kion_project" "kion_project_9" {
  ou_id = 11
}
`
	out, _, err := rewriteref.Rewrite([]byte(in), "generated.tf", index(t), refs(t))
	require.NoError(t, err)

	got := string(out)
	assert.Contains(t, got, "# __generated__ by Terraform")
	assert.Contains(t, got, `from "9"`)
	assert.True(t, strings.HasPrefix(got, "# __generated__"))
}

func TestBuildIndexRejectsGarbage(t *testing.T) {
	_, err := rewriteref.BuildIndex([]byte("import {"), "imports.tf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing imports.tf")
}

func TestRewriteRejectsGarbage(t *testing.T) {
	_, _, err := rewriteref.Rewrite([]byte("resource {"), "generated.tf", index(t), refs(t))
	require.Error(t, err)
}
