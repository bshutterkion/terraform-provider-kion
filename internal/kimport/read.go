// Package kimport enumerates a live Kion install into Terraform import
// blocks. This file walks each manifest resource per its ReadShape and builds
// the Terraform import id for every record it finds.
package kimport

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-kion/internal/kgen/importmanifest"
)

// Record is one importable object.
type Record struct {
	ID   string
	Name string
	Raw  map[string]any
}

// Result is the outcome for one resource type. Every manifest row produces one,
// so a resource is never silently absent from a run.
type Result struct {
	TFType  string
	Status  string // ok | empty | unsupported | error
	Records []Record
	Reason  string
}

// Enumerate walks one resource per its manifest row.
func Enumerate(ctx context.Context, l Lister, r importmanifest.Resource) Result {
	res := Result{TFType: r.TFType, Status: "unsupported", Reason: r.Reason}
	if !r.Readable {
		return res
	}

	switch r.ReadShape {
	case importmanifest.ShapeGeneric, importmanifest.ShapeSpecial:
		raw, err := l.List(ctx, r.ListPath)
		if err != nil {
			// Some resources codegen classifies as ordinary flat-list entities
			// (ShapeGeneric) but whose flat endpoint 405s on real installs; the
			// manifest attaches a Parent block to those as a fallback. Only take
			// it when the flat list actually failed, and only report the
			// fallback's outcome when it succeeds -- otherwise the original
			// flat-list error is what the operator needs to see.
			if r.Parent != nil {
				fallback := parentScopedResult(ctx, l, r)
				if fallback.Status != "error" {
					fallback.Reason = joinReasons(
						fmt.Sprintf("flat list failed (%v); read parent-scoped instead", err),
						fallback.Reason,
					)
					return fallback
				}
			}
			res.Status, res.Reason = "error", err.Error()
			return res
		}
		records, skipped := toRecords(raw, r, "")
		res.Records = records
		if skipped > 0 {
			res.Reason = fmt.Sprintf("%d record(s) skipped: no id", skipped)
		}
	case importmanifest.ShapeParentList, importmanifest.ShapeAssociation:
		if r.Parent == nil {
			raw, err := l.List(ctx, r.ListPath)
			if err != nil {
				res.Status, res.Reason = "error", err.Error()
				return res
			}
			records, skipped := toRecords(raw, r, "")
			res.Records = records
			if skipped > 0 {
				res.Reason = fmt.Sprintf("%d record(s) skipped: no id", skipped)
			}
			break
		}
		return parentScopedResult(ctx, l, r)
	default:
		return res
	}

	if len(res.Records) == 0 {
		res.Status = "empty"
	} else {
		res.Status = "ok"
	}
	return res
}

// parentScopedResult lists r.Parent.ListPath for the parent ids, then reads
// r.Parent.ChildPath under each one. One bad parent doesn't sink the whole
// resource -- its failure is recorded and enumeration continues -- but if
// every parent failed and nothing came back, the resource is reported as
// errored rather than merely empty.
func parentScopedResult(ctx context.Context, l Lister, r importmanifest.Resource) Result {
	res := Result{TFType: r.TFType}

	parents, err := l.List(ctx, r.Parent.ListPath)
	if err != nil {
		res.Status, res.Reason = "error", fmt.Sprintf("listing parents: %v", err)
		return res
	}

	var failures []string
	skipped := 0
	for _, parent := range parents {
		pid := stringify(parent["id"])
		if pid == "" {
			continue
		}
		path := strings.ReplaceAll(r.Parent.ChildPath, "{parent_id}", pid)
		children, err := l.List(ctx, path)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, child := range children {
			if _, ok := child[r.Parent.ParentIDField]; !ok {
				child[r.Parent.ParentIDField] = parent["id"]
			}
		}
		recs, sk := toRecords(children, r, pid)
		res.Records = append(res.Records, recs...)
		skipped += sk
	}

	var reasonParts []string
	if len(failures) > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d parent(s) failed; first: %s", len(failures), failures[0]))
	}
	if skipped > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d record(s) skipped: no id", skipped))
	}
	res.Reason = strings.Join(reasonParts, "; ")

	if len(res.Records) == 0 && len(failures) > 0 {
		res.Status = "error"
		return res
	}
	if len(res.Records) == 0 {
		res.Status = "empty"
	} else {
		res.Status = "ok"
	}
	return res
}

// joinReasons combines a leading reason with an optional trailing one.
func joinReasons(lead, rest string) string {
	if rest == "" {
		return lead
	}
	return lead + "; " + rest
}

// toRecords builds the Terraform import id for each raw object, per the
// manifest's IDFormat -- which mirrors the ImportState the crud templates
// generate for that archetype. Records with no usable id are skipped and
// counted rather than assigned a colliding synthetic id: the singleton
// fallback (deriving the id from the resource's kind) only makes sense for
// genuine ShapeSpecial singletons such as kion_app_config, which have
// exactly one record and no id at all. Applying it to any other shape would
// give every id-less record in a list the same import id.
func toRecords(raw []map[string]any, r importmanifest.Resource, parentID string) ([]Record, int) {
	out := make([]Record, 0, len(raw))
	skipped := 0
	for _, obj := range raw {
		fields := unwrapTypedRecord(obj)

		var id string
		switch r.ImportID.Format {
		case importmanifest.FormatParentSlashKey:
			key := stringify(fields[r.ImportID.KeyField])
			if parentID == "" || key == "" {
				skipped++
				continue
			}
			id = parentID + "/" + key
		default:
			id = stringify(fields["id"])
			if id == "" {
				if r.ReadShape != importmanifest.ShapeSpecial {
					skipped++
					continue
				}
				// A singleton has no id; its type is its identity.
				id = strings.TrimPrefix(r.TFType, "kion_")
			}
		}

		name := ""
		if r.NameField != "" {
			name = stringify(fields[r.NameField])
		}
		// obj (not fields) is kept as Raw so no information -- including the
		// owner_users/owner_user_groups siblings -- is lost, even when the
		// record was wrapped.
		out = append(out, Record{ID: id, Name: name, Raw: obj})
	}
	return out, skipped
}

// unwrapTypedRecord detects the per-type wrapper shape some /v3/* list
// endpoints use, e.g.:
//
//	/v3/cft             -> {"cft":{...,"id":296}, "owner_users":[...], "owner_user_groups":[...], "tags":[...]}
//	/v3/service-catalog -> {"service_catalog_portfolio":{...,"id":4}, "owner_users":[...], "owner_user_groups":[...]}
//	/v3/gcp-iam-role    -> {"gcp_role":{...,"id":9}, "car_restricted_users":[...], "car_restricted_ugroups":[...], "owner_users":[...], "owner_user_groups":[...]}
//
// The wrapper key is NOT always the resource's kind -- /v3/service-catalog
// wraps under "service_catalog_portfolio" (not "service_catalog"), and
// /v3/gcp-iam-role wraps under "gcp_role" (not "gcp_iam_role") -- and the
// sibling keys alongside the wrapper vary by type (owner_users/
// owner_user_groups, but also tags, compliance_programs,
// car_restricted_users/car_restricted_ugroups, is_enabled, ...), so neither
// the wrapper key nor the sibling set can be hard-coded or derived from the
// tf_type.
//
// Instead, detection is purely structural: if obj has no usable top-level
// "id", and exactly one of its keys maps to an object that itself has an
// "id", that inner object is the record. Any other shape -- a usable
// top-level id, no qualifying key, or more than one -- is left untouched;
// being conservative here means an ambiguous shape is skipped downstream (no
// "id") rather than mis-extracted.
func unwrapTypedRecord(obj map[string]any) map[string]any {
	if stringify(obj["id"]) != "" {
		return obj
	}
	var candidate map[string]any
	matches := 0
	for _, v := range obj {
		inner, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if stringify(inner["id"]) == "" {
			continue
		}
		matches++
		candidate = inner
	}
	if matches != 1 {
		return obj
	}
	return candidate
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
