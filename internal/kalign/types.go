// Package kalign aligns Terraform resource schemas against the kion-sdk-go
// generated client types by matching a schema model's `tfsdk:` tags to an SDK
// struct's `json:` tags. It powers two jobs that share one alignment:
//
//   - Check (#1): report drift — schema attributes with no matching SDK field,
//     incompatible primitive types, or a needed flex converter that is missing.
//   - Gen (#2): emit the flatten (SDK -> Framework) flex converters for the
//     aligned fields, so the conversion layer is generated rather than
//     hand-maintained.
//
// The resource<->SDK-type correspondence is resolved by content overlap (the SDK
// struct whose json tags best cover the model's tfsdk tags), so no hand-kept
// mapping is required.
package kalign

// SDKField is one field of an ogen-generated SDK struct.
type SDKField struct {
	GoName string // Go field name, e.g. "CreateUserID"
	JSON   string // json tag, e.g. "create_user_id"
	GoType string // Go type as source text, e.g. "OptNilUint64"
}

// ModelField is one field of a tfplugingen-generated *Model struct.
type ModelField struct {
	GoName string // Go field name, e.g. "CreateUserId"
	TFSDK  string // tfsdk tag, e.g. "create_user_id"
	TFType string // Framework type, e.g. "types.Int64"
}

// ServiceModel is a resource's schema model parsed from *_schema_gen.go.
type ServiceModel struct {
	Service string // service dir name, e.g. "ou_note"
	Name    string // struct name, e.g. "OuNoteModel"
	File    string // source path
	Fields  []ModelField
}

// Pair is one aligned attribute: a model field matched to an SDK field.
type Pair struct {
	Model    ModelField
	SDK      SDKField
	FlexFn   string // flex.<name>ToFramework for the flatten direction
	HaveFlex bool   // whether FlexFn exists in the flex package
	Nested   bool   // model field is a nested object/list (no primitive converter)
}

// Resolved is the alignment of one ServiceModel to its best-matching SDK type,
// plus the drift findings discovered while aligning.
type Resolved struct {
	Model         ServiceModel
	SDKType       string // chosen SDK struct name, e.g. "OUNote"
	Overlap       int    // count of tfsdk tags matched by that SDK type's json tags
	LowConfidence bool   // few fields overlapped; the type match is unreliable
	Pairs         []Pair
	MissingInSDK  []string // model attrs with no json match in the chosen SDK type
	TypeMismatch  []string // matched but Framework vs SDK primitive families disagree
	MissingFlex   []string // needed flex converter that does not exist
	NestedAttrs   []string // nested model attrs needing a nested converter
}

// Findings counts the drift signals in a Resolved (0 means fully aligned).
func (r Resolved) Findings() int {
	return len(r.MissingInSDK) + len(r.TypeMismatch) + len(r.MissingFlex)
}

// Source supplies the parsed inputs the aligner needs. The production
// implementation reads Go source from disk (see NewFileSource); tests use a
// generated mock so orchestration can be exercised without a filesystem.
type Source interface {
	// SDKStructs returns SDK typeName -> ordered fields, parsed from the SDK's
	// oas_schemas_gen.go at sdkFile.
	SDKStructs(sdkFile string) (map[string][]SDKField, error)
	// ServiceModels returns the *Model structs under root. If only is non-empty,
	// it is limited to that single service directory.
	ServiceModels(root, only string) ([]ServiceModel, error)
	// FlexFuncs returns the set of top-level function names defined in the flex
	// package directory.
	FlexFuncs(dir string) (map[string]bool, error)
}
