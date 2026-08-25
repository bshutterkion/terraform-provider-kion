package crud

import (
	"cmp"
	_ "embed"
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

//go:embed entity_compound.gtpl
var entityCompoundTmpl string

//go:embed datasource_compound.gtpl
var dataSourceCompoundTmpl string

// archetype is a compound-key / parent-read declaration from
// codegen/crud_archetypes.yaml, the shape the op-set and SDK AST cannot
// express on their own (see the file's header comment).
type archetype struct {
	Kind          string   `yaml:"kind"`
	ParentIDField string   `yaml:"parent_id_field"`
	ChildIDField  string   `yaml:"child_id_field"`
	ChildIDParam  string   `yaml:"child_id_param"`
	Collection    string   `yaml:"collection"`
	RecordIDField string   `yaml:"record_id_field"`
	JSONFields    []string `yaml:"json_fields"`
	// Asymmetric 2-param delete (kind: entity). See ResourceModel.DeleteRecordParam.
	DeleteRecordParam string `yaml:"delete_record_param"`
	DeleteExtraParam  string `yaml:"delete_extra_param"`
	DeleteExtraField  string `yaml:"delete_extra_field"`
	// Create-under-parent (kind: entity): the create op takes a parent-id path
	// param sourced from a model attribute, while read/update/delete address the
	// record by its own id (e.g. idms open-id access-rule / group-association).
	CreateParentParam string `yaml:"create_parent_param"` // SDK create-params field naming the parent id
	CreateParentField string `yaml:"create_parent_field"` // model attribute holding the parent id
	// Association (kind: association), one list-member row keyed by KeyField,
	// scoped by an optional ParentField, over a bulk read + replace-list API.
	KeyField     string   `yaml:"key_field"`
	ParentField  string   `yaml:"parent_field"`
	MemberFields []string `yaml:"member_fields"`
	// Parent-list (kind: parent_list), a parent-scoped resource with no by-id
	// GET; the read lists children under the parent and finds by id.
	ParentParam  string `yaml:"parent_param"`  // SDK param naming the parent id (all ops)
	ChildParam   string `yaml:"child_param"`   // SDK param naming the record id (update/delete)
	RecordType   string `yaml:"record_type"`   // the list element struct, e.g. ProjectEnforcement
	ResponseType string `yaml:"response_type"` // the read response struct, e.g. ProjectEnforcementResponse
	// Sweeper (kind: entity): the data-source collection is parent-scoped, so it
	// takes a required parent-id param the data source cannot supply. A sweeper
	// can: it enumerates SweepParent's own collection first. Without these the
	// resource gets no sweeper at all.
	SweepParent      string `yaml:"sweep_parent"`       // parent resource name, e.g. "ou"
	SweepParentParam string `yaml:"sweep_parent_param"` // list param taking the parent id, e.g. "ID"
}

const (
	compoundKindParentRead = "compound_key_parent_read"
	// entityKind routes to the normal entity path but applies archetype tweaks
	// (currently an asymmetric 2-param delete).
	entityKind = "entity"
)

// loadArchetypes reads codegen/crud_archetypes.yaml. A missing file yields an
// empty map (no resource uses the compound archetype).
func loadArchetypes(path string) (map[string]archetype, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]archetype{}, nil
		}
		return nil, fmt.Errorf("reading archetypes %s: %w", path, err)
	}
	var m map[string]archetype
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parsing archetypes %s: %w", path, err)
	}
	return m, nil
}

// bind is one TF-model <-> SDK-field binding rendered as a full Go expression.
type bind struct {
	SDKField string // SDK struct field, e.g. "StartMonth" (body binds)
	ModelGo  string // model field, e.g. "StartMonth" (flatten binds)
	Expr     string // the complete RHS Go expression
}

// dsCompoundAttr is one data-source schema attribute. The key fields
// (parent/child id) are Required inputs; the rest are Computed.
type dsCompoundAttr struct {
	TFName     string
	SchemaType string // "Int64Attribute" | "StringAttribute" | "BoolAttribute"
	Required   bool
	CustomType string // "jsontypes.NormalizedType{}" for json fields, else ""
}

// compoundData is the template payload for a compound-key/parent-read resource.
type compoundData struct {
	Pkg, Pascal, Model string
	SDKAlias           string
	ResConst, ResName  string
	DSConst, DSName    string
	Gated              bool
	TypeName           string // "kion_scope_criteria"

	IDGo       string // composite-id model field ("Id")
	ParentIDGo string // "ScopeId"
	ParentIDTF string // "scope_id"
	ChildIDGo  string // "CriteriaId"
	ChildIDTF  string // "criteria_id"

	// Parent read (extract child from a collection).
	ReadMethod    string // "GetScopeByID"
	ReadParams    string // "GetScopeByIDParams"
	ReadParentArg string // "ID"
	RespType      string // "ScopeResponse"
	RespDataPtr   bool   // envelope Data is *Payload (false = Opt-wrapper)
	Collection    string // "CriteriaRecords"
	CollectionNil bool   // collection is an OptNil<T>Array wrapper (.Value)
	RecordType    string // "ScopeCriteriaRecord"
	RecordIDGo    string // "ID"

	// Create.
	CreateMethod    string
	CreateBody      string // "ScopeCriteriaCreate"
	CreateBodyPtr   bool
	CreateParams    string
	CreateParentArg string
	CreateBinds     []bind

	// Update (optional).
	HasUpdate       bool
	UpdateMethod    string
	UpdateBody      string
	UpdateBodyPtr   bool
	UpdateParams    string
	UpdateParentArg string
	UpdateChildArg  string
	UpdateBinds     []bind

	// Delete.
	DeleteMethod    string
	DeleteParams    string
	DeleteParentArg string
	DeleteChildArg  string

	// Flatten (record -> model).
	Flattens []bind

	// Data-source schema attributes (key fields Required, rest Computed).
	DSAttrs []dsCompoundAttr

	NeedsJx bool // a json_field is present (import go-faster/jx)
}

// resolveCompound assembles a compoundData from the op-set, the SDK index, the
// archetype declaration, and the generated model. It refuses (error) any shape
// it can't map so the caller logs a skip rather than emit broken code.
func resolveCompound(name string, ops resOps, idx sdkIndex, arch archetype, model []ModelField, gated bool) (compoundData, error) {
	pascal := pascalCase(name)
	d := compoundData{
		Pkg:      name,
		Pascal:   pascal,
		Model:    pascal + "Model",
		SDKAlias: "generated",
		ResConst: "ResName" + pascal,
		ResName:  pascal + " Resource",
		DSConst:  "DSName" + pascal,
		DSName:   pascal + " Data Source",
		Gated:    gated,
		TypeName: "kion_" + name,
	}

	// Model fields: locate the composite id, parent id, and child id; the rest
	// are body/flatten payload fields.
	byTF := map[string]ModelField{}
	for _, mf := range model {
		byTF[mf.TFSDK] = mf
		switch mf.TFSDK {
		case "id":
			d.IDGo = mf.GoName
		case arch.ParentIDField:
			d.ParentIDGo, d.ParentIDTF = mf.GoName, mf.TFSDK
		case arch.ChildIDField:
			d.ChildIDGo, d.ChildIDTF = mf.GoName, mf.TFSDK
		}
	}
	if d.IDGo == "" || d.ParentIDGo == "" || d.ChildIDGo == "" {
		return d, fmt.Errorf("%s: model missing id/%s/%s", name, arch.ParentIDField, arch.ChildIDField)
	}
	jsonField := map[string]bool{}
	for _, jf := range arch.JSONFields {
		jsonField[jf] = true
		d.NeedsJx = true
	}

	// Parent read op + envelope (Scope). resolveOp("read") gives the payload
	// (Scope) fields; the archetype names the child collection within them.
	read, err := resolveOp("read", ops.Read, idx)
	if err != nil {
		return d, err
	}
	if read == nil {
		return d, fmt.Errorf("%s: compound archetype requires a read (parent) op", name)
	}
	d.ReadMethod = read.Method.Name
	d.ReadParams = read.Method.ParamsType
	d.RespType = read.RespType
	d.RespDataPtr = read.RespDataPtr
	parentArg, _, err := mapParams(read.Params, arch.ChildIDParam)
	if err != nil {
		return d, fmt.Errorf("%s read params: %w", name, err)
	}
	d.ReadParentArg = parentArg

	// The collection field on the parent payload, and its element (record) type.
	var collType string
	for _, f := range read.RespFields {
		if f.GoName == arch.Collection {
			collType = f.Type
		}
	}
	if collType == "" {
		return d, fmt.Errorf("%s: parent payload %s has no %s field", name, read.RespPayload, arch.Collection)
	}
	elem, nilAware, ok := wrapperSliceElem(collType, idx)
	if !ok {
		return d, fmt.Errorf("%s: collection %s (%s) is not a slice", name, arch.Collection, collType)
	}
	d.Collection, d.CollectionNil, d.RecordType, d.RecordIDGo = arch.Collection, nilAware, elem, arch.RecordIDField

	recStruct, ok := idx.structs[elem]
	if !ok {
		return d, fmt.Errorf("%s: record type %s not found", name, elem)
	}

	// Create.
	create, err := resolveOp("create", ops.Create, idx)
	if err != nil {
		return d, err
	}
	if create == nil || create.Body == nil {
		return d, fmt.Errorf("%s: compound archetype requires a create op with a body", name)
	}
	d.CreateMethod = create.Method.Name
	d.CreateBody = create.Method.BodyType
	d.CreateBodyPtr = create.Method.BodyPtr
	d.CreateParams = create.Method.ParamsType
	if d.CreateParentArg, _, err = mapParams(create.Params, arch.ChildIDParam); err != nil {
		return d, fmt.Errorf("%s create params: %w", name, err)
	}
	if d.CreateBinds, err = bodyBindsCompound(create.Body, byTF, jsonField); err != nil {
		return d, fmt.Errorf("%s create body: %w", name, err)
	}

	// Update (optional).
	update, err := resolveOp("update", ops.Update, idx)
	if err != nil {
		return d, err
	}
	if update != nil && update.Body != nil {
		d.HasUpdate = true
		d.UpdateMethod = update.Method.Name
		d.UpdateBody = update.Method.BodyType
		d.UpdateBodyPtr = update.Method.BodyPtr
		d.UpdateParams = update.Method.ParamsType
		if d.UpdateParentArg, d.UpdateChildArg, err = mapParams(update.Params, arch.ChildIDParam); err != nil {
			return d, fmt.Errorf("%s update params: %w", name, err)
		}
		if d.UpdateChildArg == "" {
			return d, fmt.Errorf("%s update params %s lack the child id %q", name, d.UpdateParams, arch.ChildIDParam)
		}
		if d.UpdateBinds, err = bodyBindsCompound(update.Body, byTF, jsonField); err != nil {
			return d, fmt.Errorf("%s update body: %w", name, err)
		}
	}

	// Delete.
	del, err := resolveOp("delete", ops.Delete, idx)
	if err != nil {
		return d, err
	}
	if del == nil {
		return d, fmt.Errorf("%s: compound archetype requires a delete op", name)
	}
	d.DeleteMethod = del.Method.Name
	d.DeleteParams = del.Method.ParamsType
	if d.DeleteParentArg, d.DeleteChildArg, err = mapParams(del.Params, arch.ChildIDParam); err != nil {
		return d, fmt.Errorf("%s delete params: %w", name, err)
	}
	if d.DeleteChildArg == "" {
		return d, fmt.Errorf("%s delete params %s lack the child id %q", name, d.DeleteParams, arch.ChildIDParam)
	}

	// Flatten binds (record -> model): the child id comes from the record's
	// id field; every other non-key model field maps by json/tfsdk name.
	recByJSON := map[string]Field{}
	for _, f := range recStruct.Fields {
		recByJSON[f.JSONName] = f
	}
	for _, mf := range model {
		switch mf.TFSDK {
		case "id", arch.ParentIDField:
			continue
		case arch.ChildIDField:
			rf, ok := fieldByGo(recStruct, arch.RecordIDField)
			if !ok {
				return d, fmt.Errorf("%s: record %s has no id field %s", name, elem, arch.RecordIDField)
			}
			conv, ok := flattenConverter(rf)
			if !ok {
				return d, fmt.Errorf("%s: record id field type %q unsupported", name, rf.Type)
			}
			d.Flattens = append(d.Flattens, bind{ModelGo: mf.GoName, Expr: conv + "(rec." + rf.GoName + ")"})
			continue
		}
		rf, ok := recByJSON[mf.TFSDK]
		if !ok {
			return d, fmt.Errorf("%s: record %s has no field for model %q", name, elem, mf.TFSDK)
		}
		if jsonField[mf.TFSDK] {
			d.Flattens = append(d.Flattens, bind{ModelGo: mf.GoName, Expr: "jsontypes.NewNormalizedValue(string(rec." + rf.GoName + "))"})
			continue
		}
		conv, ok := flattenConverter(rf)
		if !ok {
			return d, fmt.Errorf("%s: record field %q type %q unsupported", name, mf.TFSDK, rf.Type)
		}
		d.Flattens = append(d.Flattens, bind{ModelGo: mf.GoName, Expr: conv + "(rec." + rf.GoName + ")"})
	}

	// Data-source schema attributes: the parent + child ids are Required inputs;
	// every other model field is Computed.
	for _, mf := range model {
		st, ct, ok := dsSchemaType(mf.Type)
		if !ok {
			return d, fmt.Errorf("%s: model field %q type %q unsupported for data source", name, mf.TFSDK, mf.Type)
		}
		d.DSAttrs = append(d.DSAttrs, dsCompoundAttr{
			TFName:     mf.TFSDK,
			SchemaType: st,
			CustomType: ct,
			Required:   mf.TFSDK == arch.ParentIDField || mf.TFSDK == arch.ChildIDField,
		})
	}
	slices.SortFunc(d.DSAttrs, func(a, b dsCompoundAttr) int { return cmp.Compare(a.TFName, b.TFName) })

	return d, nil
}

// dsSchemaType maps a framework model type to its data-source schema.Attribute
// kind and (for a framework custom type) the CustomType expression.
func dsSchemaType(modelType string) (schemaType, customType string, ok bool) {
	switch modelType {
	case "types.Int64":
		return "Int64Attribute", "", true
	case "types.String":
		return "StringAttribute", "", true
	case "types.Bool":
		return "BoolAttribute", "", true
	case "jsontypes.Normalized":
		return "StringAttribute", "jsontypes.NormalizedType{}", true
	}
	return "", "", false
}

// bodyBindsCompound maps a request body's fields to model fields by json/tfsdk
// name. json_fields become jx.Raw(plan.X.ValueString()); scalars use the
// flex expand converters.
func bodyBindsCompound(body *Struct, byTF map[string]ModelField, jsonField map[string]bool) ([]bind, error) {
	var binds []bind
	for _, f := range body.Fields {
		mf, ok := byTF[f.JSONName]
		if !ok {
			continue // body field with no model attribute (e.g. server-set), skip
		}
		if jsonField[f.JSONName] {
			binds = append(binds, bind{SDKField: f.GoName, Expr: "jx.Raw(plan." + mf.GoName + ".ValueString())"})
			continue
		}
		conv, ok := expandConverter(f)
		if !ok {
			return nil, fmt.Errorf("body field %q type %q unsupported", f.JSONName, f.Type)
		}
		binds = append(binds, bind{SDKField: f.GoName, Expr: conv + "(plan." + mf.GoName + ")"})
	}
	return binds, nil
}

// mapParams splits a params struct into its parent-id arg and (optional)
// child-id arg. The child arg is the field named childParam; the parent arg is
// the single remaining field.
func mapParams(p *Struct, childParam string) (parent, child string, err error) {
	if p == nil {
		return "", "", fmt.Errorf("op has no params struct")
	}
	for _, f := range p.Fields {
		if f.GoName == childParam {
			child = f.GoName
			continue
		}
		if parent != "" {
			return "", "", fmt.Errorf("params %s has multiple non-child id fields (%s, %s)", p.Name, parent, f.GoName)
		}
		parent = f.GoName
	}
	if parent == "" {
		return "", "", fmt.Errorf("params %s has no parent id field", p.Name)
	}
	return parent, child, nil
}

func fieldByGo(s Struct, goName string) (Field, bool) {
	for _, f := range s.Fields {
		if f.GoName == goName {
			return f, true
		}
	}
	return Field{}, false
}

func renderCompoundEntity(d compoundData) ([]byte, error) {
	return execGoTemplate("entity_compound", entityCompoundTmpl, d, d.Pkg+".go")
}

func renderCompoundDataSource(d compoundData) ([]byte, error) {
	return execGoTemplate("datasource_compound", dataSourceCompoundTmpl, d, d.Pkg+"_data_source.go")
}
