// Package kimport enumerates a live Kion install into Terraform import
// blocks. This file walks each manifest resource per its ReadShape and builds
// the Terraform import id for every record it finds.
package kimport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	case importmanifest.ShapeNestedCollection:
		return nestedCollectionResult(ctx, l, r)
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

// parentSetOutcome is the raw tally from reading one parent set (every
// parent's child collection under one Parent block). It stays unrendered --
// no Reason, no Status -- because multiParentScopedResult needs to combine
// several sets' outcomes before it can decide those: whether an absence is
// worth mentioning depends on the record count across ALL parent sets, not
// just this one (see readParentSet's doc comment).
type parentSetOutcome struct {
	records        []Record
	failures       []string // real (non-404) child-read failures, as text
	absentParents  int      // parents whose child read 404'd: this parent has none
	parentsSkipped int      // parents with no usable id
	skippedNoID    int      // toRecords: records with no id
	skippedNoKey   int      // toRecords: FormatParentSlashKey records missing their key field
}

// readParentSet lists p.ListPath for the parent ids, then reads p.ChildPath
// under each one. One bad parent doesn't sink the whole set -- its failure
// is recorded in the outcome and enumeration continues.
//
// A child read that 404s is not a failure: it means this particular parent
// has none of resource r, same as a 200 with an empty body -- some resources
// (e.g. kion_idms_open_id_access_rule) only exist under a minority of
// parents, and the API answers "not found" rather than "here are zero"
// for the rest. That is counted as absentParents, separately from failures,
// so a real error (e.g. 502) is never confused with an expected gap.
//
// Only the top-level list call (p.ListPath itself failing) is returned as an
// error -- that's not "one bad parent," it's not knowing what the parents
// are at all.
func readParentSet(ctx context.Context, l Lister, p importmanifest.Parent, r importmanifest.Resource) (parentSetOutcome, error) {
	parents, err := l.List(ctx, p.ListPath)
	if err != nil {
		return parentSetOutcome{}, err
	}

	var out parentSetOutcome
	for _, parent := range parents {
		// A parent list can use the per-type wrapper shape too (the exact
		// shape unwrapTypedRecord exists for) -- unwrap before reading id,
		// or every parent silently drops with no count and no reason.
		// unwrapTypedRecord's kind-match step is for the CHILD resource r,
		// which isn't the parent's own kind -- p carries no reliable
		// tf_type-derived kind for the parent entity, so pass "" and let it
		// fall through to the structural "exactly one id-bearing map" rule,
		// which already handles this shape correctly.
		fields := unwrapTypedRecord(parent, "")
		pid := stringify(fields["id"])
		if pid == "" {
			out.parentsSkipped++
			continue
		}
		path := strings.ReplaceAll(p.ChildPath, "{parent_id}", pid)
		children, err := l.List(ctx, path)
		if err != nil {
			var statusErr *StatusError
			if errors.As(err, &statusErr) && statusErr.Status == http.StatusNotFound {
				out.absentParents++
				continue
			}
			out.failures = append(out.failures, err.Error())
			continue
		}
		for _, child := range children {
			if _, ok := child[p.ParentIDField]; !ok {
				child[p.ParentIDField] = fields["id"]
			}
		}
		recs, noID, noKey := toRecords(children, r, pid)
		out.records = append(out.records, recs...)
		out.skippedNoID += noID
		out.skippedNoKey += noKey
	}
	return out, nil
}

// parentSetReasonParts renders o's failure/skip counts as Reason fragments.
// The absence count is deliberately not one of them -- callers decide
// whether it is worth mentioning against the record count they have
// visibility into (see readParentSet's doc comment), and append it
// themselves.
func parentSetReasonParts(o parentSetOutcome) []string {
	var parts []string
	if len(o.failures) > 0 {
		parts = append(parts, fmt.Sprintf("%d parent(s) failed; first: %s", len(o.failures), o.failures[0]))
	}
	if o.parentsSkipped > 0 {
		parts = append(parts, fmt.Sprintf("%d parent(s) skipped: no id", o.parentsSkipped))
	}
	if sr := skipReason(o.skippedNoID, o.skippedNoKey); sr != "" {
		parts = append(parts, sr)
	}
	return parts
}

// parentScopedResult reads r.Parent's single parent set and renders it as a
// Result. If every parent failed (for real -- not merely had none) and
// nothing came back, the resource is reported as errored rather than merely
// empty; if the only thing standing between here and records was absent
// parents, it is empty with a Reason naming how many.
func parentScopedResult(ctx context.Context, l Lister, r importmanifest.Resource) Result {
	res := Result{TFType: r.TFType}

	outcome, err := readParentSet(ctx, l, *r.Parent, r)
	if err != nil {
		res.Status, res.Reason = "error", fmt.Sprintf("listing parents: %v", err)
		return res
	}
	res.Records = outcome.records

	parts := parentSetReasonParts(outcome)
	// Absences are worth reporting only when they explain an empty result --
	// when records also came back, a resource existing under a minority of
	// parents is the normal case, and calling it out every run would train
	// people to ignore the caveats block.
	if outcome.absentParents > 0 && len(res.Records) == 0 {
		parts = append(parts, fmt.Sprintf("%d parent(s) had none", outcome.absentParents))
	}
	res.Reason = strings.Join(parts, "; ")

	// This also fires when failures are only partial and the surviving
	// parents legitimately had zero children -- there is no way from here to
	// tell "every parent that responded had nothing" from "the responses
	// that mattered all failed," so this errs loud on purpose rather than
	// risk reporting a resource as cleanly empty when part of its read
	// actually failed. Absent (404) parents don't trigger this -- they are
	// a definite answer, not an unknown.
	switch {
	case len(res.Records) > 0:
		res.Status = "ok"
	case len(outcome.failures) > 0:
		res.Status = "error"
	default:
		res.Status = "empty"
	}
	return res
}

// multiParentScopedResult reads every parent set in r.Parents (e.g.
// kion_budget under both /v3/ou and /v3/project) and concatenates their
// records. One parent set failing entirely -- or one parent within a set
// failing -- does not prevent the other sets from being read; each set's
// failure/skip reason is carried forward, prefixed with that set's Kind so a
// multi-set failure is still traceable to which parent it came from.
// Absences are tallied across every set and, per the same rule
// parentScopedResult applies, mentioned only when the combined record count
// across all sets is zero. RenderImports already dedups by (tfType, id)
// across all of a resource's records, so no dedup happens here.
func multiParentScopedResult(ctx context.Context, l Lister, r importmanifest.Resource) Result {
	res := Result{TFType: r.TFType}

	var reasonParts []string
	anyFailure := false
	totalAbsent := 0
	for i := range r.Parents {
		p := r.Parents[i]
		outcome, err := readParentSet(ctx, l, p, r)
		if err != nil {
			anyFailure = true
			reasonParts = append(reasonParts, fmt.Sprintf("%s: listing parents: %v", p.Kind, err))
			continue
		}
		res.Records = append(res.Records, outcome.records...)
		totalAbsent += outcome.absentParents
		if len(outcome.failures) > 0 {
			anyFailure = true
		}
		if parts := parentSetReasonParts(outcome); len(parts) > 0 {
			reasonParts = append(reasonParts, fmt.Sprintf("%s: %s", p.Kind, strings.Join(parts, "; ")))
		}
	}
	if totalAbsent > 0 && len(res.Records) == 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d parent(s) had none", totalAbsent))
	}
	res.Reason = strings.Join(reasonParts, "; ")

	switch {
	case len(res.Records) > 0:
		res.Status = "ok"
	case anyFailure:
		res.Status = "error"
	default:
		res.Status = "empty"
	}
	return res
}

// nestedCollectionResult reads r.ListPath once and, for every parent object,
// extracts its records from the array nested under r.Collection -- rather
// than making a second HTTP call per parent id, the way readParentSet does
// for ShapeParentList/ShapeAssociation. Kion's /beta/scope, for example,
// returns scope objects, each carrying a criteria_records array; the
// criteria live inline in the scope's own payload and have no endpoint of
// their own.
//
// Each nested element's import id is "<parent>/<child>", both read off the
// ELEMENT itself via r.ParentIDField/r.ChildIDField -- the observed payload
// carries scope_id directly on each criteria record, so there is no need to
// fall back to the enclosing parent's own id. When an element lacks
// r.ChildIDField (live criteria records carry "id", not "criteria_id"), the
// element's own "id" is used instead; toRecords' FormatParentSlashKey branch
// has no equivalent fallback because in that shape the child id field is
// always populated -- this fallback exists here specifically because
// crud_archetypes.yaml's child_id_field for scope_criteria names a field the
// live API doesn't actually send.
//
// Only the top-level r.ListPath read can fail outright; a malformed or
// missing nested collection on one parent just yields zero records for that
// parent, same as any other resource with a legitimately empty result.
func nestedCollectionResult(ctx context.Context, l Lister, r importmanifest.Resource) Result {
	res := Result{TFType: r.TFType}

	parents, err := l.List(ctx, r.ListPath)
	if err != nil {
		res.Status, res.Reason = "error", err.Error()
		return res
	}

	var skippedNoID int
	for _, parent := range parents {
		// Same reasoning as readParentSet's unwrap call: kind is the CHILD
		// resource's kind, not the parent's, so pass "" here and let the
		// structural fallback find the parent's own id-bearing shape.
		fields := unwrapTypedRecord(parent, "")
		arr, _ := fields[r.Collection].([]any)
		for _, item := range arr {
			child, ok := item.(map[string]any)
			if !ok {
				skippedNoID++
				continue
			}
			cfields := unwrapTypedRecord(child, r.Kind)

			parentKey := stringify(cfields[r.ParentIDField])
			childKey := stringify(cfields[r.ChildIDField])
			if childKey == "" {
				childKey = stringify(cfields["id"])
			}
			if parentKey == "" || childKey == "" {
				skippedNoID++
				continue
			}

			name := ""
			if r.NameField != "" {
				name = stringify(cfields[r.NameField])
			}
			// child (not cfields) is kept as Raw so no information is lost, even
			// when the element was wrapped -- mirrors toRecords' obj-vs-fields
			// choice for Raw.
			res.Records = append(res.Records, Record{ID: parentKey + "/" + childKey, Name: name, Raw: child})
		}
	}

	res.Reason = skipReason(skippedNoID, 0)
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
