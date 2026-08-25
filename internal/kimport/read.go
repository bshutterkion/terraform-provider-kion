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
		records, skippedNoID, skippedNoKey := toRecords(raw, r, "")
		res.Records = records
		res.Reason = skipReason(skippedNoID, skippedNoKey)
	case importmanifest.ShapeParentList, importmanifest.ShapeAssociation:
		if len(r.Parents) > 0 {
			return multiParentScopedResult(ctx, l, r)
		}
		if r.Parent == nil {
			raw, err := l.List(ctx, r.ListPath)
			if err != nil {
				res.Status, res.Reason = "error", err.Error()
				return res
			}
			records, skippedNoID, skippedNoKey := toRecords(raw, r, "")
			res.Records = records
			res.Reason = skipReason(skippedNoID, skippedNoKey)
			break
		}
		return parentScopedResult(ctx, l, r)
	default:
		// A manifest row with a ReadShape this switch doesn't recognize is a
		// codegen/manifest bug, not an expected gap -- name it rather than
		// falling through with whatever (possibly empty) Reason the manifest
		// happened to carry, which would report as an unexplained blank line.
		if res.Reason == "" {
			res.Reason = fmt.Sprintf("unrecognized read shape %q", r.ReadShape)
		}
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
	skippedNoID := 0
	skippedNoKey := 0
	parentsSkipped := 0
	for _, parent := range parents {
		// A parent list can use the per-type wrapper shape too (the exact
		// shape unwrapTypedRecord exists for) -- unwrap before reading id,
		// or every parent silently drops with no count and no reason.
		// unwrapTypedRecord's kind-match step is for the CHILD resource r,
		// which isn't the parent's own kind -- r.Parent carries no reliable
		// tf_type-derived kind for the parent entity, so pass "" and let it
		// fall through to the structural "exactly one id-bearing map" rule,
		// which already handles this shape correctly.
		fields := unwrapTypedRecord(parent, "")
		pid := stringify(fields["id"])
		if pid == "" {
			parentsSkipped++
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
				child[r.Parent.ParentIDField] = fields["id"]
			}
		}
		recs, noID, noKey := toRecords(children, r, pid)
		res.Records = append(res.Records, recs...)
		skippedNoID += noID
		skippedNoKey += noKey
	}

	var reasonParts []string
	if len(failures) > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d parent(s) failed; first: %s", len(failures), failures[0]))
	}
	if parentsSkipped > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d parent(s) skipped: no id", parentsSkipped))
	}
	if sr := skipReason(skippedNoID, skippedNoKey); sr != "" {
		reasonParts = append(reasonParts, sr)
	}
	res.Reason = strings.Join(reasonParts, "; ")

	// This also fires when failures are only partial and the surviving
	// parents legitimately had zero children -- there is no way from here to
	// tell "every parent that responded had nothing" from "the responses
	// that mattered all failed," so this errs loud on purpose rather than
	// risk reporting a resource as cleanly empty when part of its read
	// actually failed.
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

// multiParentScopedResult reads every parent set in r.Parents (e.g.
// kion_budget under both /v3/ou and /v3/project) via parentScopedResult and
// concatenates their records. One parent set failing entirely -- or one
// parent within a set failing -- does not prevent the other sets from being
// read; each set's Reason (parentScopedResult already folds per-parent
// failures into it) is carried forward, prefixed with that set's Kind so a
// multi-set failure is still traceable to which parent it came from.
// RenderImports already dedups by (tfType, id) across all of a resource's
// records, so no dedup happens here.
func multiParentScopedResult(ctx context.Context, l Lister, r importmanifest.Resource) Result {
	res := Result{TFType: r.TFType}

	var reasonParts []string
	anyFailure := false
	for i := range r.Parents {
		sub := r
		p := r.Parents[i]
		sub.Parent = &p
		sub.Parents = nil

		subRes := parentScopedResult(ctx, l, sub)
		res.Records = append(res.Records, subRes.Records...)
		if subRes.Status == "error" {
			anyFailure = true
		}
		if subRes.Reason != "" {
			reasonParts = append(reasonParts, fmt.Sprintf("%s: %s", p.Kind, subRes.Reason))
		}
	}
	res.Reason = strings.Join(reasonParts, "; ")

	switch {
	case len(res.Records) == 0 && anyFailure:
		res.Status = "error"
	case len(res.Records) == 0:
		res.Status = "empty"
	default:
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
//
// Two skip counts come back rather than one: a FormatParentSlashKey record
// missing its key field is a different, more actionable cause (the
// manifest's KeyField is probably wrong for this endpoint) than a record
// missing an id outright, and collapsing both into "no id" would mislabel it.
func toRecords(raw []map[string]any, r importmanifest.Resource, parentID string) (out []Record, skippedNoID, skippedNoKey int) {
	out = make([]Record, 0, len(raw))
	for _, obj := range raw {
		fields := unwrapTypedRecord(obj, r.Kind)

		var id string
		switch r.ImportID.Format {
		case importmanifest.FormatParentSlashKey:
			if parentID == "" {
				skippedNoID++
				continue
			}
			key := stringify(fields[r.ImportID.KeyField])
			if key == "" {
				skippedNoKey++
				continue
			}
			id = parentID + "/" + key
		default:
			id = stringify(fields["id"])
			// Kion ids are positive. A literal 0 is an absent id rendered as a
			// number, and emitting it produced an import block for a record that
			// does not exist ("Cannot import non-existent remote object").
			if id == "0" {
				id = ""
			}
			if id == "" && r.ImportID.KeyField != "" {
				// association.gtpl's ImportState {{else}} branch (no parent)
				// parses req.ID as a plain integer and assigns it straight to
				// the key field -- so for a parentless association (e.g.
				// kion_global_permission_mapping, shaped
				// {"app_role_id":1,"user_ids":[1],...} with no "id" at all)
				// the import id IS the key field's value.
				id = stringify(fields[r.ImportID.KeyField])
			}
			if id == "" {
				if r.ReadShape != importmanifest.ShapeSpecial {
					skippedNoID++
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
	return out, skippedNoID, skippedNoKey
}

// skipReason renders toRecords' two skip counts as a Reason fragment,
// distinguishing a record with no id from a FormatParentSlashKey record
// missing its key field -- see toRecords' doc comment.
func skipReason(skippedNoID, skippedNoKey int) string {
	var parts []string
	if skippedNoID > 0 {
		parts = append(parts, fmt.Sprintf("%d record(s) skipped: no id", skippedNoID))
	}
	if skippedNoKey > 0 {
		parts = append(parts, fmt.Sprintf("%d record(s) skipped: missing key field", skippedNoKey))
	}
	return strings.Join(parts, "; ")
}

// unwrapTypedRecord detects the per-type wrapper shape some /v3/* and /v4/*
// list endpoints use, e.g.:
//
//	/v3/cft                       -> {"cft":{...,"id":296}, "owner_users":[...], "owner_user_groups":[...], "tags":[...]}
//	/v3/service-catalog           -> {"service_catalog_portfolio":{...,"id":4}, "owner_users":[...], "owner_user_groups":[...]}
//	/v3/gcp-iam-role              -> {"gcp_role":{...,"id":9}, "car_restricted_users":[...], "car_restricted_ugroups":[...], "owner_users":[...], "owner_user_groups":[...]}
//	/v4/ou-cloud-access-role      -> {"ou_cloud_access_role":{...,"id":5}, "ou":{...,"id":1}, "aws_iam_policies":[...], ...}
//	/v4/project-cloud-access-role -> {"project_cloud_access_role":{...,"id":7}, "project":{...,"id":2}, "users":[...], ...}
//
// kind is the resource's codegen kind (tf_type minus the "kion_" prefix,
// i.e. Resource.Kind); the wrapper key usually -- but not always -- matches
// it. When it does, that is authoritative: try it first, before the
// structural fallback below.
//
// That ordering matters because the CAR endpoints above carry TWO
// id-bearing sibling maps (the CAR itself, and its parent ou/project) --
// under the old purely-structural rule ("exactly one key maps to an object
// with an id") both are ambiguous and get skipped as id-less, and if the tie
// ever broke the other way it would silently import the *parent's* id
// instead of the record's own. Checking obj[kind] first resolves the
// ambiguity correctly by construction.
//
// The wrapper key is NOT always the resource's kind, though -- /v3/service-catalog
// wraps under "service_catalog_portfolio" (not "service_catalog"), and
// /v3/gcp-iam-role wraps under "gcp_role" (not "gcp_iam_role") -- and the
// sibling keys alongside the wrapper vary by type (owner_users/
// owner_user_groups, but also tags, compliance_programs,
// car_restricted_users/car_restricted_ugroups, is_enabled, ...), so for
// those (and whenever kind is "" -- e.g. the caller is unwrapping a parent
// record with no reliable kind to hand, see the parentScopedResult call
// site) detection falls back to the purely structural rule: if obj has no
// usable top-level "id", and exactly one of its keys maps to an object that
// itself has an "id", that inner object is the record. Any other shape -- a
// usable top-level id, no qualifying key, or more than one -- is left
// untouched; being conservative here means an ambiguous shape is skipped
// downstream (no "id") rather than mis-extracted.
func unwrapTypedRecord(obj map[string]any, kind string) map[string]any {
	if stringify(obj["id"]) != "" {
		return obj
	}
	if kind != "" {
		if inner, ok := obj[kind].(map[string]any); ok && stringify(inner["id"]) != "" {
			return inner
		}
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
