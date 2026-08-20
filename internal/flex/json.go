package flex

import (
	"github.com/go-faster/jx"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
)

// Converters between a Terraform jsontypes.Normalized attribute (a JSON string
// compared for semantic equality) and the SDK's jx.Raw (raw JSON bytes).

// NormalizedFromFramework converts a jsontypes.Normalized to jx.Raw.
// Null or unknown values produce a nil jx.Raw.
func NormalizedFromFramework(v jsontypes.Normalized) jx.Raw {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return jx.Raw(v.ValueString())
}

// NormalizedToFramework converts jx.Raw to a jsontypes.Normalized.
// Empty raw JSON produces a null value.
func NormalizedToFramework(v jx.Raw) jsontypes.Normalized {
	if len(v) == 0 {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(v))
}
