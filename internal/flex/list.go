package flex

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- Expand: TF → SDK ---

// Uint64SliceFromFramework converts a types.List of Int64 to []uint64.
// Null or unknown lists return nil.
func Uint64SliceFromFramework(ctx context.Context, v types.List) ([]uint64, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}

	var elems []types.Int64
	diags := v.ElementsAs(ctx, &elems, false)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]uint64, len(elems))
	for i, e := range elems {
		result[i] = uint64(e.ValueInt64())
	}
	return result, nil
}

// StringSliceFromFramework converts a types.List of String to []string.
// Null or unknown lists return nil.
func StringSliceFromFramework(ctx context.Context, v types.List) ([]string, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}

	var elems []types.String
	diags := v.ElementsAs(ctx, &elems, false)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]string, len(elems))
	for i, e := range elems {
		result[i] = e.ValueString()
	}
	return result, nil
}

// StringSliceFromFrameworkSet converts a types.Set of String to []string.
// Null or unknown sets return nil.
func StringSliceFromFrameworkSet(ctx context.Context, v types.Set) ([]string, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}

	var elems []types.String
	diags := v.ElementsAs(ctx, &elems, false)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]string, len(elems))
	for i, e := range elems {
		result[i] = e.ValueString()
	}
	return result, nil
}

// --- Flatten: SDK → TF ---

// Uint64SliceToFramework converts a []uint64 to a types.List of Int64.
// Nil slices produce a null types.List.
func Uint64SliceToFramework(ctx context.Context, v []uint64) (types.List, diag.Diagnostics) {
	if v == nil {
		return types.ListNull(types.Int64Type), nil
	}

	elems := make([]types.Int64, len(v))
	for i, val := range v {
		elems[i] = types.Int64Value(int64(val))
	}
	return types.ListValueFrom(ctx, types.Int64Type, elems)
}

// StringSliceToFramework converts a []string to a types.List of String.
// Nil slices produce a null types.List.
func StringSliceToFramework(ctx context.Context, v []string) (types.List, diag.Diagnostics) {
	if v == nil {
		return types.ListNull(types.StringType), nil
	}

	elems := make([]types.String, len(v))
	for i, val := range v {
		elems[i] = types.StringValue(val)
	}
	return types.ListValueFrom(ctx, types.StringType, elems)
}

// StringSliceToFrameworkSet converts a []string to a types.Set of String.
// Nil slices produce a null types.Set.
func StringSliceToFrameworkSet(ctx context.Context, v []string) (types.Set, diag.Diagnostics) {
	if v == nil {
		return types.SetNull(types.StringType), nil
	}

	elems := make([]types.String, len(v))
	for i, val := range v {
		elems[i] = types.StringValue(val)
	}
	return types.SetValueFrom(ctx, types.StringType, elems)
}

// Uint64SliceFromFrameworkSet converts a types.Set (of int64) to []uint64.
// Null or unknown sets produce a nil slice.
func Uint64SliceFromFrameworkSet(ctx context.Context, v types.Set) ([]uint64, diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	var elems []types.Int64
	diags := v.ElementsAs(ctx, &elems, false)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]uint64, len(elems))
	for i, e := range elems {
		result[i] = uint64(e.ValueInt64())
	}
	return result, nil
}

// Uint64SliceToFrameworkSet converts a []uint64 to a types.Set (of int64).
// A nil slice produces a null set.
func Uint64SliceToFrameworkSet(ctx context.Context, v []uint64) (types.Set, diag.Diagnostics) {
	if v == nil {
		return types.SetNull(types.Int64Type), nil
	}
	elems := make([]types.Int64, len(v))
	for i, val := range v {
		elems[i] = types.Int64Value(int64(val))
	}
	return types.SetValueFrom(ctx, types.Int64Type, elems)
}

// --- Membership diff: for associations synced via paired add/remove endpoints ---

// Uint64ListDiff extracts the old (state) and new (plan) uint64 id lists and
// returns the ids to add (in new, not old) and to remove (in old, not new).
// Used by resources whose owner/member associations are updated through
// separate add/remove endpoints rather than the main update body.
func Uint64ListDiff(ctx context.Context, old, cur types.List, diags *diag.Diagnostics) (add, remove []uint64) {
	o, d := Uint64SliceFromFramework(ctx, old)
	diags.Append(d...)
	n, d2 := Uint64SliceFromFramework(ctx, cur)
	diags.Append(d2...)
	return subtractUint64(n, o), subtractUint64(o, n)
}

// Uint64SetDiff is Uint64ListDiff for types.Set-typed membership attributes.
func Uint64SetDiff(ctx context.Context, old, cur types.Set, diags *diag.Diagnostics) (add, remove []uint64) {
	o, d := Uint64SliceFromFrameworkSet(ctx, old)
	diags.Append(d...)
	n, d2 := Uint64SliceFromFrameworkSet(ctx, cur)
	diags.Append(d2...)
	return subtractUint64(n, o), subtractUint64(o, n)
}

// subtractUint64 returns the elements of a that are not in b (set difference).
func subtractUint64(a, b []uint64) []uint64 {
	inB := make(map[uint64]struct{}, len(b))
	for _, x := range b {
		inB[x] = struct{}{}
	}
	var out []uint64
	for _, x := range a {
		if _, ok := inB[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}

// Int64ListDiff / Int64SetDiff are the []int64 analogs of the uint64 ones,
// for member endpoints that take []int64 (e.g. UpdateUGroupUsers).
func Int64ListDiff(ctx context.Context, old, cur types.List, diags *diag.Diagnostics) (add, remove []int64) {
	o, d := Uint64SliceFromFramework(ctx, old)
	diags.Append(d...)
	n, d2 := Uint64SliceFromFramework(ctx, cur)
	diags.Append(d2...)
	return toInt64(subtractUint64(n, o)), toInt64(subtractUint64(o, n))
}

// Int64SetDiff is Int64ListDiff for types.Set-typed member attributes.
func Int64SetDiff(ctx context.Context, old, cur types.Set, diags *diag.Diagnostics) (add, remove []int64) {
	o, d := Uint64SliceFromFrameworkSet(ctx, old)
	diags.Append(d...)
	n, d2 := Uint64SliceFromFrameworkSet(ctx, cur)
	diags.Append(d2...)
	return toInt64(subtractUint64(n, o)), toInt64(subtractUint64(o, n))
}

func toInt64(v []uint64) []int64 {
	if v == nil {
		return nil
	}
	out := make([]int64, len(v))
	for i, x := range v {
		out[i] = int64(x)
	}
	return out
}
