package crud

import (
	"bytes"
	"go/parser"
	"go/token"
	"testing"
)

func labelTestValues() testValues {
	return testValues{
		Create: map[string]string{"color": "#0088ff", "key": "test-acc-%[1]s", "value": "test-acc-%[1]s"},
		Update: map[string]string{"color": "#ff0000", "key": "test-acc-%[1]s-upd", "value": "test-acc-%[1]s-upd"},
	}
}

func TestRenderResourceTest_label(t *testing.T) {
	got, err := renderResourceTest(labelResourceModel(t), labelTestValues())
	if err != nil {
		t.Fatalf("renderResourceTest: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "label_test.go", got, parser.ParseComments); err != nil {
		t.Fatalf("generated resource test does not parse: %v\n%s", err, got)
	}
	wants := []string{
		"package label_test",
		"func TestAccKionLabel_basic(",
		"func TestAccKionLabel_update(",
		"testAccCheckLabelDestroy(ctx)",
		"testAccCheckLabelExists(ctx, resourceName)",
		"conn.Client.GetLabel(ctx, generated.GetLabelParams{ID: id})",
		"ImportStateVerify: true",
		`resource "kion_label" "test"`,
		`key = "test-acc-%[1]s"`,
		"fmt.Sprintf(",
	}
	for _, w := range wants {
		if !bytes.Contains(got, []byte(w)) {
			t.Errorf("generated resource test missing %q", w)
		}
	}
}

func TestRenderDataSourceTest_label(t *testing.T) {
	got, err := renderDataSourceTest(labelResourceModel(t), labelTestValues())
	if err != nil {
		t.Fatalf("renderDataSourceTest: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "label_data_source_test.go", got, parser.ParseComments); err != nil {
		t.Fatalf("generated data source test does not parse: %v\n%s", err, got)
	}
	wants := []string{
		"func TestAccKionLabelDataSource_basic(",
		`data.kion_label.test`,
		`data "kion_label" "test"`,
		"id = kion_label.test.id",
	}
	for _, w := range wants {
		if !bytes.Contains(got, []byte(w)) {
			t.Errorf("generated data source test missing %q", w)
		}
	}
}
