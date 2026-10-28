package crud

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestBindListedIn(t *testing.T) {
	model := []ModelField{
		{GoName: "OpenIdId", TFSDK: "open_id_id", Type: "types.Int64"},
		{GoName: "Name", TFSDK: "name", Type: "types.String"},
	}
	got, err := bindListedIn(listedInDecl{Path: "/v4/idms/open-id/{id}/group-association", ParentField: "open_id_id"}, model)
	if err != nil {
		t.Fatal(err)
	}
	want := listedInBind{PathFmt: "/v4/idms/open-id/%d/group-association", ParentGo: "OpenIdId", ParentTF: "open_id_id"}
	if *got != want {
		t.Fatalf("got %+v, want %+v", *got, want)
	}

	for name, decl := range map[string]listedInDecl{
		"no placeholder":   {Path: "/v4/x/group-association", ParentField: "open_id_id"},
		"two placeholders": {Path: "/v4/{a}/x/{b}", ParentField: "open_id_id"},
		"missing field":    {Path: "/v4/{id}/x", ParentField: "nope"},
		"non-int field":    {Path: "/v4/{id}/x", ParentField: "name"},
		"empty path":       {ParentField: "open_id_id"},
	} {
		if _, err := bindListedIn(decl, model); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func listedInLabel(t *testing.T) ResourceModel {
	t.Helper()
	rm := labelResourceModel(t)
	rm.ListedIn = &listedInBind{PathFmt: "/v4/p/%d/c", ParentGo: "Color", ParentTF: "color"}
	return rm
}

// The by-id read is not trusted on its own: a record its parent no longer lists
// is gone, whatever the by-id GET says.
func TestRenderEntity_listedIn(t *testing.T) {
	got, err := renderEntity(listedInLabel(t))
	if err != nil {
		t.Fatalf("renderEntity: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", got, parser.ParseComments); err != nil {
		t.Fatalf("does not parse: %v\n%s", err, got)
	}
	read := string(got[bytes.Index(got, []byte(") Read(")):bytes.Index(got, []byte(") Update("))])
	for _, w := range []string{
		"if !state.Color.IsNull() && !state.Color.IsUnknown() {",
		`r.Meta().RawListContainsID(ctx, fmt.Sprintf("/v4/p/%d/c", state.Color.ValueInt64()), state.Id.ValueString())`,
		"if !listed {",
	} {
		if !strings.Contains(read, w) {
			t.Errorf("Read missing %q\n%s", w, read)
		}
	}
	if strings.Index(read, "RawListContainsID") < strings.Index(read, "errs.IsNotFound(out)") {
		t.Error("list check must follow the by-id not-found check")
	}

	plain, err := renderEntity(labelResourceModel(t))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plain, []byte("RawListContainsID")) {
		t.Error("list check rendered without listed_in")
	}
}

// The acceptance checks probe the way Read does, so a destroy that only
// soft-deletes is still verified.
func TestRenderResourceTest_listedIn(t *testing.T) {
	got, err := renderResourceTest(listedInLabel(t), labelTestValues())
	if err != nil {
		t.Fatalf("renderResourceTest: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x_test.go", got, parser.ParseComments); err != nil {
		t.Fatalf("does not parse: %v\n%s", err, got)
	}
	s := string(got)
	exists := s[strings.Index(s, "Exists(_ context.Context"):strings.Index(s, "Destroy(_ context.Context")]
	destroy := s[strings.Index(s, "Destroy(_ context.Context"):]
	for _, part := range []string{exists, destroy} {
		for _, w := range []string{
			`rs.Primary.Attributes["color"]`,
			`conn.RawListContainsID(ctx, fmt.Sprintf("/v4/p/%d/c", parent), rs.Primary.ID)`,
		} {
			if !strings.Contains(part, w) {
				t.Errorf("missing %q in\n%s", w, part)
			}
		}
	}
	if !strings.Contains(exists, "not listed under its parent") {
		t.Error("Exists must fail on a record its parent does not list")
	}

	plain, err := renderResourceTest(labelResourceModel(t), labelTestValues())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plain, []byte("RawListContainsID")) {
		t.Error("list probe rendered without listed_in")
	}
}
