package kalign

import (
	"fmt"
	"sort"
	"strings"
)

// Resolve aligns one ServiceModel to the SDK type whose json tags best cover the
// model's tfsdk tags, then records drift: attributes with no SDK field, primitive
// type mismatches, and missing flex converters. flexFuncs is the set of existing
// flex function names (used to flag missing converters).
func Resolve(m ServiceModel, sdkTypes map[string][]SDKField, flexFuncs map[string]bool) Resolved {
	r := Resolved{Model: m}
	tfsdkSet := make(map[string]bool, len(m.Fields))
	for _, f := range m.Fields {
		tfsdkSet[f.TFSDK] = true
	}
	r.SDKType, r.Overlap = bestOverlapType(tfsdkSet, sdkTypes)
	if r.SDKType == "" || r.Overlap*2 < len(m.Fields) {
		r.LowConfidence = true
	}

	sdkByJSON := make(map[string]SDKField, len(sdkTypes[r.SDKType]))
	for _, f := range sdkTypes[r.SDKType] {
		sdkByJSON[f.JSON] = f
	}
	for _, mf := range m.Fields {
		sf, ok := sdkByJSON[mf.TFSDK]
		if !ok {
			r.MissingInSDK = append(r.MissingInSDK, mf.TFSDK)
			continue
		}
		p := Pair{Model: mf, SDK: sf}
		if tfFamily(mf.TFType) == "" { // nested/object/list
			p.Nested = true
			r.NestedAttrs = append(r.NestedAttrs, mf.TFSDK)
		} else {
			if !typesCompatible(mf.TFType, sf.GoType) {
				r.TypeMismatch = append(r.TypeMismatch,
					fmt.Sprintf("%s: schema %s vs SDK %s", mf.TFSDK, mf.TFType, sf.GoType))
			}
			p.FlexFn = strings.TrimPrefix(sf.GoType, "*") + "ToFramework"
			p.HaveFlex = flexFuncs[p.FlexFn]
			if !p.HaveFlex {
				r.MissingFlex = append(r.MissingFlex,
					fmt.Sprintf("%s (for field %q of type %s)", p.FlexFn, mf.TFSDK, sf.GoType))
			}
		}
		r.Pairs = append(r.Pairs, p)
	}
	return r
}

// bestOverlapType picks the SDK struct whose json tags cover the most of the
// model's tfsdk tags. Ties break lexically for determinism. Returns ("", 0) when
// nothing overlaps.
func bestOverlapType(tfsdkSet map[string]bool, sdkTypes map[string][]SDKField) (string, int) {
	names := make([]string, 0, len(sdkTypes))
	for n := range sdkTypes {
		names = append(names, n)
	}
	sort.Strings(names)
	best, bestOverlap := "", 0
	for _, name := range names {
		overlap := 0
		for _, f := range sdkTypes[name] {
			if tfsdkSet[f.JSON] {
				overlap++
			}
		}
		if overlap > bestOverlap {
			best, bestOverlap = name, overlap
		}
	}
	return best, bestOverlap
}

// typesCompatible reports whether a Framework type and an SDK Go type describe
// the same primitive family. Nested/list/object Framework types return true
// (they are handled as nested, not primitive drift).
func typesCompatible(tf, sdk string) bool {
	fam := tfFamily(tf)
	if fam == "" {
		return true
	}
	return fam == sdkFamily(sdk)
}

// tfFamily maps a terraform-plugin-framework type to a primitive family, or ""
// for nested/collection types.
func tfFamily(tf string) string {
	switch tf {
	case "types.String":
		return "string"
	case "types.Bool":
		return "bool"
	case "types.Int64", "types.Int32", "types.Number":
		return "int"
	case "types.Float64", "types.Float32":
		return "float"
	default:
		return ""
	}
}

// sdkFamily strips ogen optionality prefixes (Opt/Nil/Null) and a leading
// pointer, then maps the base type to a primitive family.
func sdkFamily(sdk string) string {
	base := strings.TrimPrefix(sdk, "*")
	for {
		switch {
		case strings.HasPrefix(base, "Opt"):
			base = base[3:]
		case strings.HasPrefix(base, "Nil"):
			base = base[3:]
		case strings.HasPrefix(base, "Null"):
			base = base[4:]
		default:
			goto done
		}
	}
done:
	base = strings.ToLower(base)
	switch {
	case base == "string":
		return "string"
	case base == "bool":
		return "bool"
	case strings.HasPrefix(base, "int") || strings.HasPrefix(base, "uint"):
		return "int"
	case strings.HasPrefix(base, "float"):
		return "float"
	default:
		return base
	}
}
