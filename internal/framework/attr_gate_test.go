package framework_test

import (
	"testing"

	"terraform-provider-kion/internal/conns"
	"terraform-provider-kion/internal/framework"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planWith builds a raw plan holding the given attributes; a nil value is an
// attribute the practitioner left unset.
func planWith(attrs map[string]*string) tfsdk.Plan {
	types := map[string]tftypes.Type{}
	vals := map[string]tftypes.Value{}
	for name, v := range attrs {
		types[name] = tftypes.String
		if v == nil {
			vals[name] = tftypes.NewValue(tftypes.String, nil)
			continue
		}
		vals[name] = tftypes.NewValue(tftypes.String, *v)
	}
	obj := tftypes.Object{AttributeTypes: types}
	return tfsdk.Plan{Raw: tftypes.NewValue(obj, vals)}
}

func meta(version string) *conns.KionClient {
	return &conns.KionClient{
		Version:         conns.MustParseKionVersion(version),
		VersionDetected: true,
	}
}

func TestRequireAttrKionVersions_errorsOnTooNewAttribute(t *testing.T) {
	set := "policy-1"
	diags := framework.RequireAttrKionVersions(
		meta("3.13.0"),
		planWith(map[string]*string{"automation_policy_ids": &set}),
		map[string]conns.KionVersion{"automation_policy_ids": conns.MustParseKionVersion("3.16.0")},
		"kion_cloud_rule",
	)
	require.True(t, diags.HasError(), "a 3.16 attribute on a 3.13 instance must error")
	assert.Contains(t, diags[0].Detail(), "kion_cloud_rule.automation_policy_ids")
	assert.Contains(t, diags[0].Detail(), "3.16.0")
	assert.Contains(t, diags[0].Detail(), "3.13.0")
}

// The silent-drop only happens for values actually sent, so an unset attribute
// must not error — otherwise every 3.13 user is blocked from the resource.
func TestRequireAttrKionVersions_ignoresUnsetAttribute(t *testing.T) {
	diags := framework.RequireAttrKionVersions(
		meta("3.13.0"),
		planWith(map[string]*string{"automation_policy_ids": nil}),
		map[string]conns.KionVersion{"automation_policy_ids": conns.MustParseKionVersion("3.16.0")},
		"kion_cloud_rule",
	)
	assert.False(t, diags.HasError(), "an unset attribute is never sent, so nothing can be dropped")
}

func TestRequireAttrKionVersions_allowsNewEnoughInstance(t *testing.T) {
	set := "policy-1"
	diags := framework.RequireAttrKionVersions(
		meta("3.16.0"),
		planWith(map[string]*string{"automation_policy_ids": &set}),
		map[string]conns.KionVersion{"automation_policy_ids": conns.MustParseKionVersion("3.16.0")},
		"kion_cloud_rule",
	)
	assert.False(t, diags.HasError())
}

func TestRequireAttrKionVersions_quietWhenVersionUndetected(t *testing.T) {
	// The resource gate already warns once; repeating it per attribute would
	// bury the warning that matters.
	set := "policy-1"
	diags := framework.RequireAttrKionVersions(
		&conns.KionClient{VersionDetected: false},
		planWith(map[string]*string{"automation_policy_ids": &set}),
		map[string]conns.KionVersion{"automation_policy_ids": conns.MustParseKionVersion("3.16.0")},
		"kion_cloud_rule",
	)
	assert.Empty(t, diags)
}

func TestRequireAttrKionVersions_noMapIsNoop(t *testing.T) {
	set := "x"
	diags := framework.RequireAttrKionVersions(
		meta("3.12.0"), planWith(map[string]*string{"anything": &set}), nil, "kion_label")
	assert.Empty(t, diags)
}
