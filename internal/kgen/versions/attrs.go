package versions

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"terraform-provider-kion/internal/kgen/crud"
)

// Resource gating asks "does this operation exist here". That misses the other
// half: an operation can exist on an older Kion while a field on its request
// body does not. Portal decodes with json.Unmarshal and no DisallowUnknownFields,
// so an unknown field is not rejected — it is dropped. The value never lands,
// the read returns without it, and Terraform reports an inconsistent result or
// diffs forever, neither of which names the real cause.
//
// deriveAttrMins answers, per resource, which tfsdk attributes exist only from
// some Kion version onward: it walks the create body struct in every tracked
// SDK version and records the earliest one carrying each field. Attributes
// present in the oldest tracked version are omitted — they need no gate.
func deriveAttrMins(src crud.Source, sdkDir, serviceRoot string, entries map[string]entry, logw io.Writer) map[string]map[string]string {
	out := map[string]map[string]string{}

	for name, e := range entries {
		if e.Create == nil {
			continue // nothing to send, nothing to gate
		}
		key := strings.ToUpper(e.Create.Method) + " " + e.Create.Path

		// Go field name -> earliest tracked version index that has it.
		earliest := map[string]int{}
		var newestFields []crud.Field

		for i, v := range trackedVersions {
			gen := filepath.Join(sdkDir, "generated", v.dir)
			methods, err := src.ClientMethods(filepath.Join(gen, "oas_client_gen.go"))
			if err != nil {
				continue // version not present locally; treated as no data
			}
			var body string
			for _, m := range methods {
				if strings.ToUpper(m.HTTPMethod)+" "+m.Path == key {
					body = strings.TrimPrefix(m.BodyType, "Opt")
					break
				}
			}
			if body == "" {
				continue // op absent in this version — resource gating covers that
			}
			structs, err := src.Structs(filepath.Join(gen, "oas_schemas_gen.go"))
			if err != nil {
				continue
			}
			st, ok := structs[body]
			if !ok {
				continue
			}
			for _, f := range st.Fields {
				if _, seen := earliest[f.GoName]; !seen {
					earliest[f.GoName] = i
				}
			}
			newestFields = st.Fields
		}
		if len(newestFields) == 0 {
			continue
		}

		// Map SDK field -> tfsdk attribute via the generated model, so the
		// diagnostic names what the practitioner actually wrote.
		pascal := pascalFor(name)
		models, err := src.ModelFields(
			filepath.Join(serviceRoot, name, name+"_schema_gen.go"), pascal+"Model")
		if err != nil {
			fmt.Fprintf(logw, "attr-versions: %s: no model (%v); skipping\n", name, err)
			continue
		}
		tfByGo := make(map[string]string, len(models))
		for _, m := range models {
			tfByGo[m.GoName] = m.TFSDK
		}

		mins := map[string]string{}
		for _, f := range newestFields {
			i, ok := earliest[f.GoName]
			if !ok || i == 0 {
				continue // present since the oldest tracked version
			}
			tf, ok := tfByGo[f.GoName]
			if !ok {
				continue // not surfaced as a Terraform attribute
			}
			mins[tf] = versionString(trackedVersions[i])
		}
		if len(mins) > 0 {
			out[name] = mins
		}
	}
	return out
}

// pascalFor converts a snake_case package name to the PascalCase prefix the
// generated model uses: ou_note -> OuNote.
func pascalFor(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// renderAttrMins renders the sorted attribute->version map literal body.
func renderAttrMins(mins map[string]string) string {
	if len(mins) == 0 {
		return ""
	}
	names := make([]string, 0, len(mins))
	for k := range mins {
		names = append(names, k)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "\t%q: conns.MustParseKionVersion(%q),\n", n, mins[n])
	}
	return b.String()
}

// pruneRedundant drops attribute minimums at or below the resource's own
// minimum. The resource gate already refuses those, and emitting both would
// report one cause twice — every field of a 3.14-only resource is trivially
// "3.14+". What remains is the interesting case: a field newer than the
// resource carrying it.
func pruneRedundant(mins map[string]string, resourceMin string) map[string]string {
	if len(mins) == 0 {
		return nil
	}
	floor := minorOf(resourceMin)
	out := make(map[string]string, len(mins))
	for attr, v := range mins {
		if minorOf(v) > floor {
			out[attr] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// minorOf extracts NN from "3.NN.0"; an unparseable or empty string is 0, so an
// ungated resource keeps every attribute minimum.
func minorOf(v string) int {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0
	}
	n := 0
	for _, r := range parts[1] {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
