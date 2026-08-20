package crud

import (
	"bytes"
	"go/parser"
	"go/token"
	"testing"
)

func TestRenderDataSource_labelDualMode(t *testing.T) {
	got, err := renderDataSource(labelResourceModel(t))
	if err != nil {
		t.Fatalf("renderDataSource: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "label_data_source.go", got, parser.ParseComments); err != nil {
		t.Fatalf("does not parse: %v\n%s", err, got)
	}
	for _, w := range []string{
		"func (d *labelDataSource) readByID(",
		"func (d *labelDataSource) readByFilter(",
		"func fetchAllLabel(ctx context.Context, conn *generated.Client)",
		"filter.Schema()",
		"filter.Match(ctx, data.Filter, labelToRow(lbl))",
		"conn.GetLabelIndex(ctx, generated.GetLabelIndexParams{",
		"generated.NewOptInt64(page)",
		"resp.Data.Value.Items",
		"resp.Data.Value.Total",
		"data.Id = flex.OptUint64ToFramework(lbl.ID)",
		"Filter []filter.Model `tfsdk:\"filter\"`",
	} {
		if !bytes.Contains(got, []byte(w)) {
			t.Errorf("dual-mode DS missing %q", w)
		}
	}
}

func TestRenderDataSource_idOnlyFallback(t *testing.T) {
	rm := labelResourceModel(t)
	rm.List = nil // simulate a resource with no list endpoint
	got, err := renderDataSource(rm)
	if err != nil {
		t.Fatalf("renderDataSource: %v", err)
	}
	if bytes.Contains(got, []byte("filter.Schema()")) {
		t.Error("id-only fallback must not reference filter")
	}
	if !bytes.Contains(got, []byte("func (d *labelDataSource) Read(")) {
		t.Error("id-only fallback missing Read")
	}
}

func TestRenderIDOnlyDataSource_label(t *testing.T) {
	got, err := renderIDOnlyDataSource(labelResourceModel(t))
	if err != nil {
		t.Fatalf("renderIDOnlyDataSource: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "label_data_source.go", got, parser.ParseComments); err != nil {
		t.Fatalf("generated data source does not parse: %v\n%s", err, got)
	}

	wants := []string{
		"func (d *labelDataSource) Read(",
		"conn.GetLabel(ctx, generated.GetLabelParams{ID: data.Id.ValueInt64()})",
		"data.Id = flex.OptUint64ToFramework(v.ID)",
		"data.Key = flex.StringToFramework(v.Key)",
		"type labelDataSourceModel struct",
		"`tfsdk:\"id\"`",
		"schema.Int64Attribute{",
	}
	for _, w := range wants {
		if !bytes.Contains(got, []byte(w)) {
			t.Errorf("generated data source missing %q", w)
		}
	}

	// Documented reduction: the id-only DS must NOT reference the legacy filter
	// block or the list/index pagination path.
	for _, unwanted := range []string{"filter.", "GetLabelIndex", "ListNestedAttribute"} {
		if bytes.Contains(got, []byte(unwanted)) {
			t.Errorf("id-only data source unexpectedly references %q", unwanted)
		}
	}
}
