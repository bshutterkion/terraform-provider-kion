package importmanifest

import "strings"

// ListPathFrom strips a trailing "/{...}" id template off a read path to get
// the list endpoint. A read path that is already a list (billing_source's
// /v4/billing-source) is returned unchanged.
func ListPathFrom(readPath string) string {
	if readPath == "" {
		return ""
	}
	if i := strings.Index(readPath, "/{"); i >= 0 {
		return readPath[:i]
	}
	return readPath
}

// Classify maps a crud_archetypes.yaml archetype onto a read shape and an
// import-id format. The format must match what the matching crud template
// generates in ImportState -- see the package doc.
func Classify(archetype string, readPath string, hasParent bool) (ReadShape, IDFormat, bool, string) {
	switch archetype {
	case "no_read":
		// "no_read" means no by-id GET, not "unreadable" -- e.g.
		// kion_aws_resource_tag has no by-id read but IS listable at a flat
		// collection endpoint (private_endpoints.yaml's list_read_only
		// section). Build resolves readPath through the same four-source
		// fallback chain it uses for every archetype (see Build's doc
		// comment); it is not narrowed specially for no_read. In practice a
		// no_read kind's generator_config.yaml resources/data_sources entries
		// are typically empty -- internal/kgen/crud/noread.go builds a
		// no_read resource's model from Create/Update/Delete only, so the
		// CRUD generator never populates a read path for it there -- which is
		// why a non-empty readPath reaching this branch usually came from
		// list_read_only or, as a last resort, private_endpoints.yaml's own
		// resources section. That is a fact about the input data, though, not
		// a rule this function or Build enforces.
		if ListPathFrom(readPath) == "" {
			return ShapeNone, FormatID, false,
				"crud_archetypes.yaml declares kind: no_read (no by-id GET) and no list-only or private read path was found either"
		}
		return ShapeGeneric, FormatID, true, ""
	case "datasource_only":
		return ShapeNone, FormatID, false,
			"crud_archetypes.yaml declares kind: datasource_only - not a managed resource"
	case "association":
		// association.gtpl's {{if .HasParent}} branch splits req.ID on "/" into
		// (ParentTF, KeyTF); the {{else}} branch treats req.ID as a plain id.
		// The format mirrors this conditional.
		if hasParent {
			return ShapeAssociation, FormatParentSlashKey, true, ""
		}
		return ShapeAssociation, FormatID, true, ""
	case "parent_list", "compound_key_parent_read":
		// Both look the record up under its parent, so both split req.ID on "/"
		// in ImportState: compound_key_parent_read always has, and parent_list
		// now does too. A bare id leaves the parent zero and every read 404s,
		// which is how 187 enforcement imports failed against a real install.
		return ShapeParentList, FormatParentSlashKey, true, ""
	case "singleton", "raw_http", "cv_override":
		return ShapeSpecial, FormatID, true, ""
	}

	if ListPathFrom(readPath) == "" {
		return ShapeNone, FormatID, false,
			"no read path in generator_config.yaml or importmanifest paths table"
	}
	return ShapeGeneric, FormatID, true, ""
}
