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
// so an unknown field is not rejected. It is dropped. The value never lands,
// the read returns without it, and Terraform reports an inconsistent result or
// diffs forever, neither of which names the real cause.
//
// deriveAttrWindows answers, per resource, the tracked-version window in which
// each tfsdk attribute is accepted. It walks the create body struct in every
// tracked SDK version and records the first and last one carrying each field.
// Attributes present for the whole life of the operation are omitted.
func deriveAttrWindows(src crud.Source, sdkDir, serviceRoot string, entries map[string]entry, logw io.Writer) map[string]map[string]attrWindow {
	out := map[string]map[string]attrWindow{}

	for name, e := range entries {
		if e.Create == nil {
			continue // nothing to send, nothing to gate
		}
		key := strings.ToUpper(e.Create.Method) + " " + e.Create.Path

		// Go field name -> first and last tracked version index carrying it.
		earliest := map[string]int{}
		latest := map[string]int{}
		// Union across versions, so a field the newest release dropped is still
		// considered.
		anyField := map[string]crud.Field{}
		// Newest version carrying the op, so a dropped attribute is not confused
		// with a dropped endpoint (the resource gate covers that).
		lastOpAt := -1

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
				continue // op absent in this version, resource gating covers that
			}
			structs, err := src.Structs(filepath.Join(gen, "oas_schemas_gen.go"))
			if err != nil {
				continue
			}
			st, ok := structs[body]
			if !ok {
				continue
			}
			lastOpAt = i
			for _, f := range st.Fields {
				if _, seen := earliest[f.GoName]; !seen {
					earliest[f.GoName] = i
				}
				latest[f.GoName] = i
				anyField[f.GoName] = f
			}
		}
		if lastOpAt < 0 {
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

		windows := map[string]attrWindow{}
		for goName := range anyField {
			first, ok := earliest[goName]
			if !ok {
				continue
			}
			last := latest[goName]
			// Present for the whole life of the operation: no gate needed.
			if first == 0 && last == lastOpAt {
				continue
			}
			tf, ok := tfByGo[goName]
			if !ok {
				continue // not surfaced as a Terraform attribute
			}
			var w attrWindow
			if first > 0 {
				w.Min = versionString(trackedVersions[first])
			}
			if last < lastOpAt {
				// Exclusive: carried through trackedVersions[last], so the bound
				// is the start of the next tracked line.
				w.Before = versionString(trackedVersions[last+1])
			}
			windows[tf] = w
		}
		if len(windows) > 0 {
			out[name] = windows
		}
	}
	return out
}

// attrWindow is one derived range: Min inclusive, Before exclusive. An empty
// value means unbounded on that side.
type attrWindow struct {
	Min    string
	Before string
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
func renderAttrMins(windows map[string]attrWindow) string {
	if len(windows) == 0 {
		return ""
	}
	names := make([]string, 0, len(windows))
	for k := range windows {
		names = append(names, k)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		w := windows[n]
		minExpr := "conns.KionVersion{}"
		if w.Min != "" {
			minExpr = fmt.Sprintf("conns.MustParseKionVersion(%q)", w.Min)
		}
		if w.Before == "" {
			fmt.Fprintf(&b, "\t%q: {{Min: %s}},\n", n, minExpr)
			continue
		}
		fmt.Fprintf(&b, "\t// Dropped in %s; sending it there is silently ignored.\n", w.Before)
		fmt.Fprintf(&b, "\t%q: {{Min: %s, Before: conns.MustParseKionVersion(%q)}},\n", n, minExpr, w.Before)
	}
	return b.String()
}

// pruneRedundant drops attribute minimums at or below the resource's own
// minimum. The resource gate already refuses those, and emitting both would
// report one cause twice, every field of a 3.14-only resource is trivially
// "3.14+". What remains is the interesting case: a field newer than the
// resource carrying it.
func pruneRedundant(windows map[string]attrWindow, resourceMin string) map[string]attrWindow {
	if len(windows) == 0 {
		return nil
	}
	floor := minorOf(resourceMin)
	out := make(map[string]attrWindow, len(windows))
	for attr, w := range windows {
		// An upper bound is never redundant with a floor.
		if w.Before != "" || minorOf(w.Min) > floor {
			out[attr] = w
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
