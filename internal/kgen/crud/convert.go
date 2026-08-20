package crud

// Converter selection is mechanical: an SDK field's Go type + optionality maps
// to exactly one internal/flex helper. expandConverter is TF->SDK (request
// bodies); flattenConverter is SDK->TF (response envelopes). Both return
// ok=false for a type the entity archetype does not handle (nested objects,
// slices, sets) — the caller then refuses the resource for a crud_override.

// expandConverter returns the flex.*FromFramework helper for a request-body field.
func expandConverter(f Field) (string, bool) {
	switch f.Type {
	case "string":
		return "flex.StringValueFromFramework", true
	case "OptString":
		return "flex.OptStringFromFramework", true
	case "int64":
		return "flex.Int64ValueFromFramework", true
	case "uint64":
		return "flex.Uint64FromFramework", true
	case "OptInt64":
		return "flex.OptInt64FromFramework", true
	case "OptUint64":
		return "flex.OptUint64FromFramework", true
	case "OptGCPRoleLaunchStage":
		return "flex.GCPRoleLaunchStageFromFramework", true
	case "OptNullString":
		return "flex.OptNullStringFromFramework", true
	case "OptNullTime":
		return "flex.OptNullTimeFromFramework", true
	case "float64":
		return "flex.Float64ValueFromFramework", true
	case "OptFloat64":
		return "flex.OptFloat64FromFramework", true
	case "bool":
		return "flex.BoolValueFromFramework", true
	case "Raw":
		return "flex.NormalizedFromFramework", true
	case "OptBool":
		return "flex.OptBoolFromFramework", true
	case "OptNilBool":
		return "flex.OptNilBoolFromFramework", true
	case "NilBool":
		return "flex.NilBoolFromFramework", true
	case "NilUint64":
		return "flex.NilUint64FromFramework", true
	case "OptNilUint64":
		return "flex.OptNilUint64FromFramework", true
	case "OptNilInt64":
		return "flex.OptNilInt64FromFramework", true
	}
	return "", false
}

// flattenConverter returns the flex.*ToFramework helper for a response field.
func flattenConverter(f Field) (string, bool) {
	switch f.Type {
	case "string":
		return "flex.StringToFramework", true
	case "OptString":
		return "flex.OptStringToFramework", true
	case "int64":
		return "flex.Int64ToFramework", true
	case "uint64":
		return "flex.Uint64ToFramework", true
	case "OptInt64":
		return "flex.OptInt64ToFramework", true
	case "OptUint64":
		return "flex.OptUint64ToFramework", true
	case "OptGCPRoleLaunchStage":
		return "flex.GCPRoleLaunchStageToFramework", true
	case "OptNullString":
		return "flex.OptNullStringToFramework", true
	case "OptNullTime":
		return "flex.OptNullTimeToFramework", true
	case "float64":
		return "flex.Float64ToFramework", true
	case "OptFloat64":
		return "flex.OptFloat64ToFramework", true
	case "bool":
		return "flex.BoolToFramework", true
	case "Raw":
		return "flex.NormalizedToFramework", true
	case "OptBool":
		return "flex.OptBoolToFramework", true
	case "OptNilBool":
		return "flex.OptNilBoolToFramework", true
	case "NilBool":
		return "flex.NilBoolToFramework", true
	case "NilUint64":
		return "flex.NilUint64ToFramework", true
	case "OptNilUint64":
		return "flex.OptNilUint64ToFramework", true
	case "OptNilInt64":
		return "flex.OptNilInt64ToFramework", true
	}
	return "", false
}

// sliceConverter returns the flex slice helpers (expand, flatten) for a slice
// body/response field. These take (ctx, …) and return diagnostics, so the
// templates emit them as a prelude rather than an inline expression. wrap names
// the ogen nil-aware wrapper (OptNil<T>Array) when the field is wrapped — the
// expand result is boxed as generated.<wrap>{Value: v, Set: true} and the
// flatten source reads its .Value; wrap is "" for a bare slice.
func sliceConverter(sdkType string) (expand, flatten, wrap string, ok bool) {
	switch sdkType {
	case "[]uint64":
		return "flex.Uint64SliceFromFramework", "flex.Uint64SliceToFramework", "", true
	case "[]string":
		return "flex.StringSliceFromFramework", "flex.StringSliceToFramework", "", true
	case "OptNilUint64Array":
		return "flex.Uint64SliceFromFramework", "flex.Uint64SliceToFramework", "OptNilUint64Array", true
	case "OptNilStringArray":
		return "flex.StringSliceFromFramework", "flex.StringSliceToFramework", "OptNilStringArray", true
	}
	return "", "", "", false
}
