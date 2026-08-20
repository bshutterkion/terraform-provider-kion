package crud

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"slices"
	"text/template"
)

//go:embed datasource.gtpl
var dataSourceTmpl string

//go:embed datasource_list.gtpl
var dataSourceListTmpl string

// dsField binds one data-source attribute: schema type, model type, flatten
// converter, and the SDK response field it reads.
type dsField struct {
	ModelGo    string // model field, e.g. "Key"
	TFName     string // tfsdk name, e.g. "key"
	SchemaType string // "StringAttribute" | "Int64Attribute" | "BoolAttribute"
	ModelType  string // "types.String" | "types.Int64" | "types.Bool"
	Flatten    string // "flex.StringToFramework"
	SDKGo      string // response field GoName, e.g. "Key"
}

// dsData is the template payload for the id-only data source.
type dsData struct {
	Pkg, Pascal, Model       string
	DSConst, DSName          string
	SDKAlias                 string
	ReadMethod, ReadParams   string
	ReadIDParam              string
	IDParamType              string // "int64" | "uint64"
	IDGo, IDFlatten, IDSDKGo string
	HasRespID                bool // response echoes the id (re-flatten it); else keep config id
	DataPtr                  bool // envelope Data is a pointer (*Payload) not an Opt-wrapper
	RespType                 string
	Attrs                    []dsField
}

// needsPayload reports whether the read response is consulted at all — false for
// a degenerate read (empty payload, no echoed id), where the data source only
// returns the config-provided id.
func (d dsData) NeedsPayload() bool { return d.HasRespID || len(d.Attrs) > 0 }

// renderDataSource renders <name>_data_source.go. When the resource has a
// resolved list endpoint it emits the full dual-mode data source (id + legacy
// filter blocks + list pagination); otherwise, or if the dual-mode build fails,
// it falls back to the id-only data source.
func renderDataSource(rm ResourceModel) ([]byte, error) {
	if rm.List != nil {
		b, err := renderListDataSource(rm)
		if err == nil {
			return b, nil
		}
		fmt.Fprintf(os.Stderr, "kgen crud: %s — dual-mode data source failed (%v); using id-only\n", rm.Name, err)
	}
	return renderIDOnlyDataSource(rm)
}

// renderIDOnlyDataSource renders an id-only <name>_data_source.go (single read
// by id, no filter/list). The degradation path for resources without a
// resolvable list endpoint.
func renderIDOnlyDataSource(rm ResourceModel) ([]byte, error) {
	data, err := buildDSData(rm)
	if err != nil {
		return nil, err
	}
	return execGoTemplate("datasource", dataSourceTmpl, data, rm.Name+"_data_source.go")
}

func buildDSData(rm ResourceModel) (dsData, error) {
	respByJSON := map[string]Field{}
	for _, f := range rm.Read.RespFields {
		respByJSON[f.JSONName] = f
	}

	d := dsData{
		Pkg:        rm.Name,
		Pascal:     rm.Pascal,
		Model:      rm.Name + "DataSourceModel",
		DSConst:    "DSName" + rm.Pascal,
		DSName:     rm.Pascal + " Data Source",
		SDKAlias:   "generated",
		ReadMethod: rm.Read.Method.Name,
		ReadParams: rm.Read.Method.ParamsType,
		RespType:   rm.Read.RespType,
		DataPtr:    rm.Read.RespDataPtr,
		IDGo:       rm.IDField.GoName,
	}

	readIDParam, readIDType, err := idParamName(rm.Read.Params)
	if err != nil {
		return d, fmt.Errorf("%s data source read: %w", rm.Name, err)
	}
	d.ReadIDParam = readIDParam
	d.IDParamType = readIDType

	// The id may be absent from the read response (some APIs don't echo it). Then
	// the data source keeps the config-provided id rather than re-flattening it.
	if idResp, ok := respByJSON["id"]; ok {
		idFlatten, ok := flattenConverter(idResp)
		if !ok {
			return d, fmt.Errorf("%s data source: id response field type %q unsupported", rm.Name, idResp.Type)
		}
		d.HasRespID = true
		d.IDFlatten = idFlatten
		d.IDSDKGo = idResp.GoName
	}

	for _, mf := range rm.Fields {
		rf, ok := respByJSON[mf.TFSDK]
		if !ok {
			continue
		}
		// The id-only data source can only expose scalar fields it knows how to
		// render; skip anything else (e.g. list/set/object) rather than refuse the
		// whole resource — the resource itself is unaffected.
		sType, ok := schemaAttrType(mf.Type)
		if !ok {
			continue
		}
		flat, ok := flattenConverter(rf)
		if !ok {
			continue
		}
		d.Attrs = append(d.Attrs, dsField{
			ModelGo:    mf.GoName,
			TFName:     mf.TFSDK,
			SchemaType: sType,
			ModelType:  mf.Type,
			Flatten:    flat,
			SDKGo:      rf.GoName,
		})
	}
	slices.SortFunc(d.Attrs, func(a, b dsField) int { return cmp.Compare(a.TFName, b.TFName) })
	return d, nil
}

// schemaAttrType maps a framework model type to its schema.Attribute kind.
func schemaAttrType(modelType string) (string, bool) {
	switch modelType {
	case "types.String":
		return "StringAttribute", true
	case "types.Int64":
		return "Int64Attribute", true
	case "types.Bool":
		return "BoolAttribute", true
	}
	return "", false
}

// execGoTemplate parses, executes, and gofmts a Go-source template.
func execGoTemplate(name, tmpl string, data any, outName string) ([]byte, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute %s template: %w", name, err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated %s: %w\n%s", outName, err, buf.Bytes())
	}
	return src, nil
}

// --- dual-mode (id + filter + list) data source ---

// listObjField describes one attribute of the `list` nested object (and its
// listObjectAttrTypes entry), plus how to build its value from an SDK element.
type listObjField struct {
	TFName     string // "id","key"
	SchemaType string // "Int64Attribute","StringAttribute" (nested schema)
	AttrType   string // "types.Int64Type","types.StringType" (object attr types)
	IsID       bool
	SDKGo      string // element field GoName: "ID","Key"
	ObjExpr    string // non-id object value: "types.StringValue(lbl.Key)"
	RowExpr    string // non-id filter-row value: "lbl.Key"
}

// listScalarAssign drives readByID (Flatten) and readByFilter (NullExpr) for the
// top-level scalar attributes.
type listScalarAssign struct {
	ModelGo  string // "Id","Key"
	Flatten  string // "flex.OptUint64ToFramework" / "flex.StringToFramework"
	SDKGo    string // "ID","Key"
	NullExpr string // "types.Int64Null()" / "types.StringNull()"
	IsID     bool
}

type listDSData struct {
	dsData
	ElemType     string // "Label" — the list element / read payload type
	ListMethod   string
	ListParams   string
	ListRespType string
	ItemsGo      string
	TotalGo      string
	PageParam    string
	CountParam   string
	ItemsNil     bool
	ObjFields    []listObjField
	Assigns      []listScalarAssign
}

func renderListDataSource(rm ResourceModel) ([]byte, error) {
	data, err := buildListDSData(rm)
	if err != nil {
		return nil, err
	}
	return execGoTemplate("datasource_list", dataSourceListTmpl, data, rm.Name+"_data_source.go")
}

func buildListDSData(rm ResourceModel) (listDSData, error) {
	base, err := buildDSData(rm)
	if err != nil {
		return listDSData{}, err
	}
	// The dual-mode list objects carry an id; without one in the response the
	// list can't expose it, so degrade to the id-only data source.
	if !base.HasRespID {
		return listDSData{}, fmt.Errorf("%s dual-mode data source: response has no id field", rm.Name)
	}
	respByJSON := map[string]Field{}
	for _, f := range rm.Read.RespFields {
		respByJSON[f.JSONName] = f
	}
	lm := rm.List
	d := listDSData{
		dsData:       base,
		ElemType:     lm.ElemType,
		ListMethod:   lm.Method.Name,
		ListParams:   lm.Method.ParamsType,
		ListRespType: lm.RespType,
		ItemsGo:      lm.ItemsGo,
		ItemsNil:     lm.ItemsNil,
		TotalGo:      lm.TotalGo,
		PageParam:    lm.PageParam,
		CountParam:   lm.CountParam,
	}

	d.ObjFields = append(d.ObjFields, listObjField{
		TFName: "id", SchemaType: "Int64Attribute", AttrType: "types.Int64Type",
		IsID: true, SDKGo: base.IDSDKGo,
	})
	d.Assigns = append(d.Assigns, listScalarAssign{
		ModelGo: base.IDGo, Flatten: base.IDFlatten, SDKGo: base.IDSDKGo,
		NullExpr: "types.Int64Null()", IsID: true,
	})

	for _, a := range base.Attrs {
		pf, ok := respByJSON[a.TFName]
		if !ok {
			return listDSData{}, fmt.Errorf("field %q not in response payload", a.TFName)
		}
		attrType, objExpr, rowExpr, ok := listValueExprs(a.SDKGo, pf.Type)
		if !ok {
			return listDSData{}, fmt.Errorf("field %q payload type %q unsupported for list", a.TFName, pf.Type)
		}
		d.ObjFields = append(d.ObjFields, listObjField{
			TFName: a.TFName, SchemaType: a.SchemaType, AttrType: attrType,
			SDKGo: a.SDKGo, ObjExpr: objExpr, RowExpr: rowExpr,
		})
		d.Assigns = append(d.Assigns, listScalarAssign{
			ModelGo: a.ModelGo, Flatten: a.Flatten, SDKGo: a.SDKGo,
			NullExpr: nullExpr(a.ModelType),
		})
	}
	return d, nil
}

// listValueExprs returns the object attr type + object/row value expressions for
// a list element scalar field. ok=false for a type the list path can't render.
func listValueExprs(sdkGo, payloadType string) (attrType, objExpr, rowExpr string, ok bool) {
	switch payloadType {
	case "string":
		return "types.StringType", "types.StringValue(lbl." + sdkGo + ")", "lbl." + sdkGo, true
	case "OptString":
		return "types.StringType", "types.StringValue(lbl." + sdkGo + `.Or(""))`, "lbl." + sdkGo + `.Or("")`, true
	case "bool":
		return "types.BoolType", "types.BoolValue(lbl." + sdkGo + ")", "lbl." + sdkGo, true
	}
	return "", "", "", false
}

func nullExpr(modelType string) string {
	switch modelType {
	case "types.Int64":
		return "types.Int64Null()"
	case "types.Bool":
		return "types.BoolNull()"
	default:
		return "types.StringNull()"
	}
}
