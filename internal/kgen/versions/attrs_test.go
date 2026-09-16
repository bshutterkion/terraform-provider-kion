package versions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneRedundant_dropsAtOrBelowResourceMin(t *testing.T) {
	// Every field of a 3.14-only resource is trivially "3.14+", the resource
	// gate already refuses those, so reporting them again is one cause twice.
	mins := map[string][]attrWindow{
		"account_alias": {{Min: "3.14.0"}},
		"account_name":  {{Min: "3.14.0"}},
		"something_new": {{Min: "3.16.0"}},
		"older_somehow": {{Min: "3.13.0"}},
	}
	got := pruneRedundant(mins, "3.14.0")
	assert.Equal(t, map[string][]attrWindow{"something_new": {{Min: "3.16.0"}}}, got)
}

func TestPruneRedundant_ungatedResourceKeepsEverything(t *testing.T) {
	// The interesting case: the resource itself has no minimum, so a newer
	// field is the only thing standing between the practitioner and a value
	// the API will silently drop.
	mins := map[string][]attrWindow{"automation_policy_ids": {{Min: "3.16.0"}}}
	assert.Equal(t, mins, pruneRedundant(mins, ""))
}

func TestPruneRedundant_emptyWhenAllRedundant(t *testing.T) {
	assert.Nil(t, pruneRedundant(map[string][]attrWindow{"name": {{Min: "3.16.0"}}}, "3.16.0"))
	assert.Nil(t, pruneRedundant(nil, "3.12.0"))
}

func TestMinorOf(t *testing.T) {
	assert.Equal(t, 16, minorOf("3.16.0"))
	assert.Equal(t, 12, minorOf("3.12.0"))
	assert.Equal(t, 0, minorOf(""), "no minimum must sort below every version")
	assert.Equal(t, 0, minorOf("garbage"))
}

func TestPascalFor(t *testing.T) {
	assert.Equal(t, "OuNote", pascalFor("ou_note"))
	assert.Equal(t, "CloudRule", pascalFor("cloud_rule"))
	assert.Equal(t, "Label", pascalFor("label"))
}

func TestRenderAttrMins_sortedAndEmpty(t *testing.T) {
	assert.Empty(t, renderAttrMins(nil))
	out := renderAttrMins(map[string][]attrWindow{"zeta": {{Min: "3.16.0"}}, "alpha": {{Min: "3.15.0"}}})
	require.NotEmpty(t, out)
	// Deterministic order, or the generated file churns between runs.
	assert.Less(t, strings.Index(out, "alpha"), strings.Index(out, "zeta"))
	assert.Contains(t, out, `"alpha": {{Min: conns.MustParseKionVersion("3.15.0")}}`)
}

// A max survives pruning at or below the resource minimum.
func TestPruneRedundant_keepsWindowsWithAnUpperBound(t *testing.T) {
	windows := map[string][]attrWindow{
		"cloud_access_role_type_id": {{Min: "3.15.0", Before: "3.16.0"}},
	}
	assert.Equal(t, windows, pruneRedundant(windows, "3.16.0"))
}

func TestRenderAttrMins_emitsUpperBound(t *testing.T) {
	out := renderAttrMins(map[string][]attrWindow{
		"cloud_access_role_type_id": {{Min: "3.15.0", Before: "3.16.0"}},
	})
	assert.Contains(t, out, `Before: conns.MustParseKionVersion("3.16.0")`)
	assert.Contains(t, out, "silently ignored")
}

// A window open at the bottom still renders, so an attribute present from the
// oldest tracked version but dropped later is expressible.
func TestRenderAttrMins_upperBoundWithoutMin(t *testing.T) {
	out := renderAttrMins(map[string][]attrWindow{"legacy": {{Before: "3.16.0"}}})
	assert.Contains(t, out, `"legacy": {{Before: conns.MustParseKionVersion("3.16.0")}}`)
	assert.NotContains(t, out, "Min:", "an unbounded floor is the zero value, not written out")
}
