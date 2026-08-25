// Package importmanifest derives, from the same codegen inputs the CRUD
// generator consumes, everything an enumerator needs to walk a live Kion
// install and emit Terraform `import` blocks: the list endpoint per resource,
// the read shape, and the import-id format.
//
// The import-id format exists nowhere else. internal/kgen/crud/*.gtpl generate
// ImportState per archetype -- entity.gtpl and parentlist.gtpl set id from
// req.ID directly, association.gtpl splits req.ID on "/" into (parent, key) --
// and a consumer outside this repo cannot know that. Deriving the manifest from
// the same archetype table that drives those templates is what keeps the two
// from diverging silently.
package importmanifest

// ManifestVersion is bumped when the JSON shape changes incompatibly.
const ManifestVersion = 2

// ReadShape is how a resource's records are enumerated.
type ReadShape string

// ReadShape constants.
const (
	ShapeGeneric     ReadShape = "generic"     // flat list at ListPath
	ShapeParentList  ReadShape = "parent_list" // one collection per parent
	ShapeAssociation ReadShape = "association" // membership rows under a parent
	ShapeSpecial     ReadShape = "special"     // bespoke endpoint (singleton/raw)
	ShapeNone        ReadShape = "none"        // cannot be enumerated
)

// IDFormat is how a record's Terraform import id is built. It mirrors the
// ImportState implementation the matching crud template generates.
type IDFormat string

// IDFormat constants.
const (
	FormatID             IDFormat = "id"               // req.ID is the record id
	FormatParentSlashKey IDFormat = "parent_slash_key" // "<parent_id>/<key_id>"
)

// Parent describes a parent-scoped or association read.
type Parent struct {
	Kind          string `json:"kind"`
	ListPath      string `json:"list_path"`
	ChildPath     string `json:"child_path"` // contains "{parent_id}"
	ParentIDField string `json:"parent_id_field"`
}

// ImportID describes how to build the id for an `import` block.
type ImportID struct {
	Format   IDFormat `json:"format"`
	KeyField string   `json:"key_field,omitempty"`
}

// Resource is one row of the manifest.
type Resource struct {
	TFType    string    `json:"tf_type"`
	Kind      string    `json:"kind"`
	Archetype string    `json:"archetype"`
	ReadShape ReadShape `json:"read_shape"`
	Readable  bool      `json:"readable"`
	ListPath  string    `json:"list_path,omitempty"`
	NameField string    `json:"name_field,omitempty"`
	Parent    *Parent   `json:"parent,omitempty"`
	// Parents holds every parent set for a resource enumerable under more than
	// one parent collection (e.g. kion_budget under both /v3/ou and
	// /v3/project). Parent always mirrors Parents[0] when Parents is set, so a
	// reader that only knows about Parent still gets a correct (if partial) read.
	Parents  []Parent `json:"parents,omitempty"`
	ImportID ImportID `json:"import_id"`
	Reason   string   `json:"reason,omitempty"`
}

// Manifest is the generated document. Resources are sorted by TFType so a
// regeneration diff means a real change.
type Manifest struct {
	Version   int        `json:"version"`
	Resources []Resource `json:"resources"`
}
