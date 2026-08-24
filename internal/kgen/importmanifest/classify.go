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

// Classify maps a crud_archetypes.yaml kind onto a read shape and an import-id
// format. The format must match what the matching crud template generates in
// ImportState -- see the package doc.
func Classify(kind, archetype, readPath string) (ReadShape, IDFormat, bool, string) {
	switch archetype {
	case "no_read":
		return ShapeNone, FormatID, false,
			"crud_archetypes.yaml declares kind: no_read - no by-id GET and no listable collection"
	case "datasource_only":
		return ShapeNone, FormatID, false,
			"crud_archetypes.yaml declares kind: datasource_only - not a managed resource"
	case "association":
		// association.gtpl splits req.ID on "/" into (ParentTF, KeyTF).
		return ShapeAssociation, FormatParentSlashKey, true, ""
	case "parent_list", "compound_key_parent_read":
		return ShapeParentList, FormatID, true, ""
	case "singleton", "raw_http", "cv_override":
		return ShapeSpecial, FormatID, true, ""
	}

	if ListPathFrom(readPath) == "" {
		return ShapeNone, FormatID, false,
			"no read path in generator_config.yaml or importmanifest paths table"
	}
	return ShapeGeneric, FormatID, true, ""
}
