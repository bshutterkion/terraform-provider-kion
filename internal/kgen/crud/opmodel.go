package crud

// ClientMethod is one SDK client method, e.g. PostLabel.
type ClientMethod struct {
	Name       string // "PostLabel"
	HTTPMethod string // "POST" (from the ogen doc-comment route)
	Path       string // "/v3/label"
	BodyType   string // "OptCreateLabel" / "OUNoteCreate"  ("" if no request body)
	BodyPtr    bool   // the body param is *T (pointer) rather than an Opt-wrapper
	ParamsType string // "GetLabelParams"  ("" if the op has no params struct)
	ResultType string // "PostLabelRes"
}

// Field is one field of an SDK body/params struct.
type Field struct {
	GoName   string // "Color"
	JSONName string // "color"        (from the json tag; "" falls back to GoName)
	Type     string // "string", "OptString", "OptUint64", "int64", ...
	Optional bool   // true iff Type is an Opt*-wrapper
	Ptr      bool   // true iff the field is a pointer (*T) — e.g. a *Payload data envelope
}

// Struct is a parsed SDK struct (a request body, a response, or a params struct).
type Struct struct {
	Name   string
	Fields []Field
}

// ModelField is one field of a generated <Name>Model (from <name>_schema_gen.go).
type ModelField struct {
	GoName string // "Color", "Id"
	TFSDK  string // "color", "id"   (from the tfsdk tag)
	Type   string // "types.String", "types.Int64", "types.Bool", "types.List", "types.Set"
}

// OpModel is a fully-resolved operation ready for templating.
type OpModel struct {
	Verb        string       // "create" | "read" | "update" | "delete"
	Method      ClientMethod //
	Body        *Struct      // request body struct (nil if none)
	Params      *Struct      // params struct (nil if none)
	RespType    string       // success response type name for flatten (read), e.g. "LabelResponse"
	RespPayload string       // domain payload type inside the envelope, e.g. "Label"
	RespDataPtr bool         // the envelope's Data field is a pointer (*Payload), not an Opt-wrapper
	RespFields  []Field      // fields of the payload inside the envelope (Data.Value.* or Data.*)
	// Record wrapper: some read payloads nest the bulk of the record under a
	// sub-object (e.g. CFTWithOwnersAndTags.cft) alongside sibling collections
	// (owners, tags), while the model is flat. These name that sub-object and its
	// fields so the flatten can source the flat model from the wrapper path.
	RespWrapperGo     string  // wrapper field Go name, e.g. "Cft" ("" when none)
	RespWrapperOpt    bool    // the wrapper field is an Opt-wrapper (access .Value)
	RespWrapperFields []Field // the wrapper struct's fields
}

// listModel describes a resolved paginated-list endpoint (the data-source read),
// enabling the dual-mode data source and the real sweeper. nil when unresolved.
type listModel struct {
	Method     ClientMethod // e.g. GetLabelIndex
	Params     *Struct      // e.g. GetLabelIndexParams
	RespType   string       // "LabelListPaginatedResponse"
	DataInner  string       // "LabelListPaginated"
	ItemsGo    string       // "Items" (field in DataInner)
	ItemsNil   bool         // items wrapper has a .Null field (OptNil*)
	ElemType   string       // "Label" (== read payload type)
	TotalGo    string       // "Total" ("" if absent)
	PageParam  string       // "Page"
	CountParam string       // "Count"
}

// ResourceModel is everything the templates need for one resource.
type ResourceModel struct {
	Name          string       // snake, e.g. "label"
	Pascal        string       // "Label"
	Model         string       // model type, e.g. "LabelModel"
	IDField       ModelField   // the tfsdk:"id" model field
	Fields        []ModelField // model fields excluding id
	Create        OpModel      //
	Read          OpModel      //
	Update        *OpModel     // nil if resource has no update
	Delete        *OpModel     // nil if resource has no delete
	List          *listModel   // nil if no resolvable list endpoint
	Gated         bool         // resource is version-gated (emit RequireKionVersionInRange)
	SchemaVersion int          // >0 when the resource migrates old SDKv2 state (bumps schema.Version)
	// Asymmetric 2-param delete (e.g. compliance_control's
	// DELETE /v4/compliance/program/{id}/control/{controlID}): the record-id
	// param plus an extra param sourced from a model field. All "" for the
	// common single-param delete.
	DeleteRecordParam string // delete param holding the record id (e.g. "ControlID")
	DeleteExtraParam  string // the other delete param (e.g. "ID")
	DeleteExtraField  string // model tfsdk field sourcing DeleteExtraParam (e.g. "program_id")
	// Create-under-parent: create's id param is the parent id from a model field
	// (not a literal). All "" for the common create. CreateParentField is the Go
	// model field name; CreateParentCast is a conversion prefix when not int64.
	CreateParentParam string
	CreateParentField string
	CreateParentCast  string
	// Nested-object fields: create/update body objects expanded to SDK structs,
	// read-payload objects flattened into tfplugingen Value types.
	CreateNested nestedResult
	UpdateNested nestedResult
	ReadNested   nestedFlatResult
	// Owner association synced on Update via paired add/remove endpoints.
	Owners *ownerMembershipBind
	// Bulk association syncs (each a struct body of id-lists via one add/remove pair).
	Assocs []*assocMembershipBind
	// Slice member syncs ([]int64 add/remove endpoints, e.g. user_group users).
	SliceMembers []*sliceMemberBind
}

// Source is the AST boundary — mockable, reads only.
type Source interface {
	// ClientMethods parses oas_client_gen.go: one entry per (c *Client) method.
	ClientMethods(clientFile string) ([]ClientMethod, error)
	// Structs parses a schemas or parameters file into name->Struct.
	Structs(file string) (map[string]Struct, error)
	// MarkerImpls maps an unexported marker-method name (e.g. "getLabelRes")
	// to the concrete types that implement it (e.g. ["LabelResponse"]).
	MarkerImpls(schemaFile string) (map[string][]string, error)
	// ModelFields parses <name>_schema_gen.go for the named model struct.
	ModelFields(schemaGenFile, modelType string) ([]ModelField, error)
}
