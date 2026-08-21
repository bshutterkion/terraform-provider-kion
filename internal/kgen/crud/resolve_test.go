package crud

import (
	"errors"
	"maps"
	"strings"
	"testing"
)

// fixtureIndex builds an sdkIndex from the Label testdata fixtures via the real
// fileSource — exercising the parsers end-to-end, not a mock.
func fixtureIndex(t *testing.T) sdkIndex {
	t.Helper()
	src := NewFileSource()
	cms, err := src.ClientMethods("testdata/oas_client_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]ClientMethod{}
	for _, m := range cms {
		methods[m.HTTPMethod+" "+m.Path] = m
	}
	schemas, err := src.Structs("testdata/oas_schemas_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	params, err := src.Structs("testdata/oas_parameters_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	structs := maps.Clone(schemas)
	maps.Copy(structs, params)
	markers, err := src.MarkerImpls("testdata/oas_schemas_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	return sdkIndex{methods: methods, structs: structs, markerImpls: markers}
}

func labelOps() resOps {
	return resOps{
		Create: &opRef{Method: "POST", Path: "/v3/label"},
		Read:   &opRef{Method: "GET", Path: "/v3/label/{id}"},
		Update: &opRef{Method: "PATCH", Path: "/v3/label/{id}"},
		Delete: &opRef{Method: "DELETE", Path: "/v3/label/{id}"},
	}
}

func labelDS() dsOps {
	return dsOps{Read: &opRef{Method: "GET", Path: "/v3/label"}}
}

func TestResolveResource_label(t *testing.T) {
	idx := fixtureIndex(t)
	model, err := NewFileSource().ModelFields("testdata/label_schema_gen.go", "LabelModel")
	if err != nil {
		t.Fatal(err)
	}

	rm, err := resolveResource("label", labelOps(), labelDS(), idx, model)
	if err != nil {
		t.Fatalf("resolveResource: %v", err)
	}

	if rm.List == nil || rm.List.Method.Name != "GetLabelIndex" {
		t.Errorf("rm.List = %+v (want resolved GetLabelIndex)", rm.List)
	}
	if rm.Pascal != "Label" || rm.Model != "LabelModel" {
		t.Errorf("rm pascal/model = %q/%q", rm.Pascal, rm.Model)
	}
	if rm.Create.Method.Name != "PostLabel" || rm.Create.Body == nil || rm.Create.Body.Name != "CreateLabel" {
		t.Errorf("create = %+v body=%+v", rm.Create.Method, rm.Create.Body)
	}
	if rm.Read.Method.Name != "GetLabel" || rm.Read.Params == nil || rm.Read.Params.Name != "GetLabelParams" {
		t.Errorf("read = %+v params=%+v", rm.Read.Method, rm.Read.Params)
	}
	if rm.Read.RespType != "LabelResponse" || len(rm.Read.RespFields) != 4 {
		t.Errorf("read envelope = %q fields=%d", rm.Read.RespType, len(rm.Read.RespFields))
	}
	if rm.Update == nil || rm.Update.Method.Name != "PatchLabel" {
		t.Errorf("update = %+v", rm.Update)
	}
	if rm.Delete == nil || rm.Delete.Method.Name != "DeleteLabel" {
		t.Errorf("delete = %+v", rm.Delete)
	}
	if rm.IDField.GoName != "Id" || len(rm.Fields) != 3 {
		t.Errorf("id=%+v fields=%d (want Id + 3 non-id)", rm.IDField, len(rm.Fields))
	}
}

func TestResolveOp_privateEndpointRefused(t *testing.T) {
	idx := fixtureIndex(t)
	_, err := resolveOp("read", &opRef{Method: "GET", Path: "/v1/dashboard/{id}"}, idx)
	var target noSDKOpError
	if !errors.As(err, &target) {
		t.Fatalf("want noSDKOpError for a route absent from the SDK, got %v", err)
	}
}

// labelRead is the resolved single-read op the list resolution is matched
// against (its payload type must equal the list element).
func labelRead(t *testing.T, idx sdkIndex) OpModel {
	t.Helper()
	op, err := resolveOp("read", &opRef{Method: "GET", Path: "/v3/label/{id}"}, idx)
	if err != nil {
		t.Fatalf("resolveOp read: %v", err)
	}
	return *op
}

// Shape A: Data wraps a struct that holds the items slice plus Total, and the
// op paginates.
func TestResolveList_shapeA_paginatedEnvelope(t *testing.T) {
	idx := fixtureIndex(t)
	lm, err := resolveList(&opRef{Method: "GET", Path: "/v3/label"}, idx, labelRead(t, idx))
	if err != nil {
		t.Fatalf("resolveList: %v", err)
	}
	if lm.Method.Name != "GetLabelIndex" {
		t.Errorf("method = %s", lm.Method.Name)
	}
	if lm.RespType != "LabelListPaginatedResponse" || lm.DataInner != "LabelListPaginated" {
		t.Errorf("resp=%s inner=%s", lm.RespType, lm.DataInner)
	}
	if lm.DataDirect || !lm.DataOpt || lm.ElemIsWrapper {
		t.Errorf("direct=%v opt=%v elemIsWrapper=%v (want false/true/false)", lm.DataDirect, lm.DataOpt, lm.ElemIsWrapper)
	}
	if lm.ItemsGo != "Items" || lm.ElemType != "Label" || !lm.ItemsNil || !lm.ItemsOpt {
		t.Errorf("items=%s elem=%s nil=%v opt=%v", lm.ItemsGo, lm.ElemType, lm.ItemsNil, lm.ItemsOpt)
	}
	if lm.TotalGo != "Total" || !lm.Paginated || lm.PageParam != "Page" || lm.CountParam != "Count" {
		t.Errorf("total=%s paginated=%v page=%s count=%s", lm.TotalGo, lm.Paginated, lm.PageParam, lm.CountParam)
	}
	a := lm.access()
	if a.EnvelopeGuard != "resp.Data.Set" || a.ItemsExpr != "resp.Data.Value.Items" || a.TotalExpr != "resp.Data.Value.Total" {
		t.Errorf("access = %+v", a)
	}
	if a.ItemsGuard != "items.Set && !items.Null" || a.ItemsSlice != "items.Value" {
		t.Errorf("access items = %+v", a)
	}
}

// Shape B: Data IS the items slice, and the op takes no pagination params. This
// is the regression: it used to abort list resolution entirely (the inner type
// was looked up as the literal string "[]Label"), silently downgrading the data
// source to id-only.
func TestResolveList_shapeB_directSliceNotPaginated(t *testing.T) {
	idx := fixtureIndex(t)
	lm, err := resolveList(&opRef{Method: "GET", Path: "/v3/label-flat"}, idx, labelRead(t, idx))
	if err != nil {
		t.Fatalf("resolveList: %v", err)
	}
	if !lm.DataDirect || lm.DataInner != "" {
		t.Errorf("direct=%v inner=%q (want true/empty)", lm.DataDirect, lm.DataInner)
	}
	if lm.ItemsGo != "Data" || lm.ElemType != "Label" || lm.ItemsOpt || lm.ItemsNil {
		t.Errorf("items=%s elem=%s opt=%v nil=%v", lm.ItemsGo, lm.ElemType, lm.ItemsOpt, lm.ItemsNil)
	}
	if lm.Paginated || lm.PageParam != "" || lm.TotalGo != "" {
		t.Errorf("paginated=%v page=%q total=%q (endpoint has neither)", lm.Paginated, lm.PageParam, lm.TotalGo)
	}
	a := lm.access()
	if a.EnvelopeGuard != "" || a.ItemsExpr != "resp.Data" || a.ItemsSlice != "items" || a.ItemsGuard != "" {
		t.Errorf("access = %+v", a)
	}
	if a.ItemsBreak != "len(items) == 0" || a.TotalExpr != "" || a.HasParams {
		t.Errorf("access = %+v", a)
	}
}

// A collection op whose params carry a required value the data source cannot
// supply (the parent id of a nested collection) is refused rather than called
// with a zero id.
func TestResolveList_requiredParamRefused(t *testing.T) {
	idx := fixtureIndex(t)
	_, err := resolveList(&opRef{Method: "GET", Path: "/v3/label/{id}/child"}, idx, labelRead(t, idx))
	if err == nil {
		t.Fatal("want refusal for a list op with a required param")
	}
	if !strings.Contains(err.Error(), "requires param") {
		t.Errorf("error should name the unsupplyable param, got %v", err)
	}
}

// An items slice of some unrelated type is not the collection for this resource.
func TestResolveList_elementTypeMismatchRefused(t *testing.T) {
	idx := fixtureIndex(t)
	_, err := resolveList(&opRef{Method: "GET", Path: "/v3/label-foreign"}, idx, labelRead(t, idx))
	if err == nil {
		t.Fatal("want refusal when the list element is not a type the read yields")
	}
}

func TestResolveList_noListOpRefused(t *testing.T) {
	idx := fixtureIndex(t)
	if _, err := resolveList(&opRef{Method: "GET", Path: "/v3/nope"}, idx, labelRead(t, idx)); err == nil {
		t.Fatal("want error for unresolved list op")
	}
}

func TestLoadConfig_skipsFlaggedAndParsesOps(t *testing.T) {
	resources, dataSources, flagged, err := loadConfig("testdata/generator_config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resources["account"]; ok {
		t.Error("commented-out INCOMPLETE resource must not be loaded")
	}
	if !flagged["guessed"] {
		t.Error("heuristic-annotated resource must be flagged")
	}
	lbl := resources["label"]
	if lbl.Create == nil || lbl.Create.Method != "POST" || lbl.Create.Path != "/v3/label" {
		t.Errorf("label create = %+v", lbl.Create)
	}
	if lbl.Delete == nil || lbl.Delete.Method != "DELETE" {
		t.Errorf("label delete = %+v", lbl.Delete)
	}
	if ds := dataSources["label"]; ds.Read == nil || ds.Read.Path != "/v3/label" {
		t.Errorf("label ds read = %+v", ds.Read)
	}
}
