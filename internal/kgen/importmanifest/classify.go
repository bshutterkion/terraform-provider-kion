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
		return ShapeNone, FormatID, false,
			"crud_archetypes.yaml declares kind: no_read - no by-id GET and no listable collection"
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
