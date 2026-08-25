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
	mins := map[string]string{
		"account_alias": "3.14.0",
		"account_name":  "3.14.0",
		"something_new": "3.16.0",
		"older_somehow": "3.13.0",
	}
	got := pruneRedundant(mins, "3.14.0")
	assert.Equal(t, map[string]string{"something_new": "3.16.0"}, got)
}

func TestPruneRedundant_ungatedResourceKeepsEverything(t *testing.T) {
	// The interesting case: the resource itself has no minimum, so a newer
	// field is the only thing standing between the practitioner and a value
	// the API will silently drop.
	mins := map[string]string{"automation_policy_ids": "3.16.0"}
	assert.Equal(t, mins, pruneRedundant(mins, ""))
}

func TestPruneRedundant_emptyWhenAllRedundant(t *testing.T) {
	assert.Nil(t, pruneRedundant(map[string]string{"name": "3.16.0"}, "3.16.0"))
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
	out := renderAttrMins(map[string]string{"zeta": "3.16.0", "alpha": "3.15.0"})
	require.NotEmpty(t, out)
	// Deterministic order, or the generated file churns between runs.
	assert.Less(t, strings.Index(out, "alpha"), strings.Index(out, "zeta"))
	assert.Contains(t, out, `"alpha": conns.MustParseKionVersion("3.15.0")`)
}
