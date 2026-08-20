package framework

import (
	"fmt"
	"sort"

	"terraform-provider-kion/internal/conns"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// RequireMinKionVersion gates a resource or data source on a minimum Kion
// version. It is a convenience wrapper around RequireKionVersionInRange with no
// upper bound.
func RequireMinKionVersion(meta *conns.KionClient, minVer conns.KionVersion, typeName string) diag.Diagnostics {
	return RequireKionVersionInRange(meta, minVer, conns.KionVersion{}, typeName)
}

// RequireKionVersionInRange gates a resource or data source on the range of Kion
// versions that support its API operations. Call it at the top of every CRUD
// method (Create/Read/Update/Delete) for any type whose operations exist only
// within a bounded window of Kion releases:
//
//	resp.Diagnostics.Append(framework.RequireKionVersionInRange(
//		r.Meta(), minKionVersion, maxKionVersion, "kion_ou_note")...)
//	if resp.Diagnostics.HasError() {
//		return
//	}
//
// A zero-value minVer or maxVer (conns.KionVersion{}) means "unbounded" on that
// side — an endpoint introduced in 3.16 and still current passes maxVer as the
// zero value; one dropped after 3.13 passes minVer 3.12 and maxVer 3.13.
//
// Behavior:
//   - detected and within [min, max]  → no diagnostics (allowed)
//   - detected and below min          → error (needs a newer Kion)
//   - detected and above max          → error (removed in a newer Kion)
//   - version undetected              → warning only (allowed; the API will
//     reject the call itself if the operation is unsupported)
func RequireKionVersionInRange(meta *conns.KionClient, minVer, maxVer conns.KionVersion, typeName string) diag.Diagnostics {
	var diags diag.Diagnostics
	hasMin := minVer != conns.KionVersion{}
	hasMax := maxVer != conns.KionVersion{}

	if meta == nil || !meta.VersionDetected {
		diags.AddWarning(
			"Kion version could not be determined",
			fmt.Sprintf(
				"%s is supported on %s. The provider could not detect the connected instance's "+
					"version, so it is proceeding; the API will reject the request if the operation "+
					"is unsupported.",
				typeName, describeRange(minVer, maxVer, hasMin, hasMax),
			),
		)
		return diags
	}

	// Below the minimum.
	if hasMin && !meta.Version.AtLeast(minVer) {
		diags.AddError(
			"Resource not supported by this Kion version",
			fmt.Sprintf(
				"%s requires Kion >= %s, but the connected instance reports %s. "+
					"Upgrade Kion or remove this resource from your configuration.",
				typeName, minVer, meta.Version,
			),
		)
		return diags
	}

	// Above the maximum (maxVer is NOT >= the instance version).
	if hasMax && !maxVer.AtLeast(meta.Version) {
		diags.AddError(
			"Resource not supported by this Kion version",
			fmt.Sprintf(
				"%s is only supported on Kion <= %s and was removed in a later release, but the "+
					"connected instance reports %s. Remove this resource from your configuration.",
				typeName, maxVer, meta.Version,
			),
		)
	}

	return diags
}

// describeRange renders a human-readable support range for warning messages.
func describeRange(minVer, maxVer conns.KionVersion, hasMin, hasMax bool) string {
	switch {
	case hasMin && hasMax:
		return fmt.Sprintf("Kion %s - %s", minVer, maxVer)
	case hasMin:
		return fmt.Sprintf("Kion >= %s", minVer)
	case hasMax:
		return fmt.Sprintf("Kion <= %s", maxVer)
	default:
		return "all Kion versions"
	}
}

// RequireAttrKionVersions reports each attribute the practitioner set that the
// connected Kion is too old to accept. Without it the request still goes out:
// the API decodes with json.Unmarshal and no DisallowUnknownFields, so an
// unrecognized field is dropped rather than rejected, and the failure surfaces
// as an inconsistent-result error or an endless diff instead of a version
// problem.
//
// It reads the raw plan rather than typed attributes so one helper covers every
// attribute type. Unset attributes are ignored — only what is actually being
// sent can be rejected.
func RequireAttrKionVersions(meta *conns.KionClient, plan tfsdk.Plan, mins map[string]conns.KionVersion, typeName string) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(mins) == 0 || meta == nil || !meta.VersionDetected {
		return diags // undetected is already warned about by the resource gate
	}
	if plan.Raw.IsNull() || !plan.Raw.IsKnown() {
		return diags
	}

	var obj map[string]tftypes.Value
	if err := plan.Raw.As(&obj); err != nil {
		return diags // not an object; nothing to inspect
	}

	names := make([]string, 0, len(mins))
	for name := range mins {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		v, ok := obj[name]
		if !ok || v.IsNull() {
			continue
		}
		want := mins[name]
		if meta.Version.AtLeast(want) {
			continue
		}
		diags.AddAttributeError(
			path.Root(name),
			"Attribute not supported by this Kion version",
			fmt.Sprintf(
				"%s.%s requires Kion %s or newer; this instance reports %s. "+
					"Remove the attribute or upgrade Kion — sending it would be silently ignored by the API.",
				typeName, name, want, meta.Version,
			),
		)
	}
	return diags
}
