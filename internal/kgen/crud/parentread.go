package crud

import (
	"fmt"
	"strings"
)

// parentRead gives a no_read resource a real Read.
//
// "no_read" means the public spec has no single-record GET, not that the record
// cannot be read: for the cloud-access-role exemptions the spec has only POST
// and DELETE, while the private /v1 collection returns the record in full. A
// resource without a Read imports as an empty shell -- ImportState sets only the
// id, the no-op Read echoes it back, and `terraform plan -generate-config-out`
// writes `resource "…" "…" {}`. The plan then reports no changes, because every
// attribute is Optional+Computed and null, so the result looks green and
// describes nothing. Every one of the 22 kion_ou_cloud_access_role_exemption
// records on a live install imported that way.
//
// Declared in codegen/private_endpoints.yaml under `parent_read`.
type parentRead struct {
	// Path is the collection to read. It may contain "{parent_id}"; when it
	// does, ParentTF must name the model attribute holding that id and the
	// import id becomes "<parent>/<id>" so an import can reach the collection.
	Path string `yaml:"path"`

	ParentTF string `yaml:"parent_tf"` // model attr holding the parent id, e.g. ou_id

	// ParentJSON is the record's own key for its owning parent, e.g. OUID.
	// The collection is inherited rather than owned -- /v1/ou/{id}/… returns
	// every exemption visible to that OU's subtree, so one record comes back
	// under many OUs and the id in the path is not its owner. Taking the parent
	// from the record is what makes the value correct and the import id stable.
	ParentJSON string `yaml:"parent_json"`

	// Require names a SQL-null-wrapper field that must be Valid for the record
	// to belong to this resource at all. The exemption collections mix cloud
	// RULE exemptions in with cloud ACCESS ROLE exemptions: of 22 records on a
	// live install only 6 were the latter, and the other 16 would have imported
	// as this type while being something else entirely.
	Require string `yaml:"require"`

	Fields []readShapeSub `yaml:"fields"` // record keys -> model attrs
}

// parentReadData is the template payload rendered into entity_noread.gtpl.
type parentReadData struct {
	Path         string
	HasParent    bool
	ParentTF     string
	ParentGo     string
	ParentJSON   string
	Require      string
	RequireGo    string
	WireStructGo string
	FlattenGo    string
	UsesFlex     bool
}

// buildParentRead renders the wire struct and flatten body for a parent_read.
func buildParentRead(pkg string, pr parentRead, byTF map[string]ModelField, idGo string) (*parentReadData, error) {
	d := &parentReadData{
		Path:       pr.Path,
		HasParent:  strings.Contains(pr.Path, "{parent_id}"),
		ParentTF:   pr.ParentTF,
		ParentJSON: pr.ParentJSON,
		Require:    pr.Require,
	}
	if d.HasParent {
		if pr.ParentTF == "" {
			return nil, fmt.Errorf("%s parent_read: path contains {parent_id} but parent_tf is empty", pkg)
		}
		mf, ok := byTF[pr.ParentTF]
		if !ok {
			return nil, fmt.Errorf("%s parent_read: parent_tf %q is not a model attribute", pkg, pr.ParentTF)
		}
		d.ParentGo = mf.GoName
		if pr.ParentJSON == "" {
			return nil, fmt.Errorf("%s parent_read: parent_tf %q needs parent_json, the record's own key for its owner", pkg, pr.ParentTF)
		}
	}

	var wire strings.Builder
	fmt.Fprintf(&wire, "type %sRecord struct {\nID int64 `json:\"id\"`\n", pkg)
	if d.HasParent {
		// flex.NullInt, not int64: the owner key is a bare number on some
		// collections (OUID) and a SQL null wrapper on others (project_id).
		fmt.Fprintf(&wire, "%s *flex.NullInt `json:%q`\n", pascalCase(pr.ParentJSON), pr.ParentJSON)
		d.UsesFlex = true
	}
	if pr.Require != "" {
		d.RequireGo = pascalCase(pr.Require)
		// The discriminator is always a SQL null wrapper; Valid is the whole point.
		fmt.Fprintf(&wire, "%s *flex.NullInt `json:%q`\n", d.RequireGo, pr.Require)
		d.UsesFlex = true
	}
	seen := map[string]bool{"id": true, pr.Require: true, pr.ParentJSON: true}
	for _, f := range pr.Fields {
		if seen[f.From] {
			continue
		}
		seen[f.From] = true
		w, err := kindWire(f.Kind)
		if err != nil {
			return nil, fmt.Errorf("%s parent_read field %q: %w", pkg, f.TF, err)
		}
		fmt.Fprintf(&wire, "%s %s `json:%q`\n", pascalCase(f.From), w, f.From)
	}
	wire.WriteString("}")
	d.WireStructGo = wire.String()

	var flat strings.Builder
	fmt.Fprintf(&flat, "m.%s = types.StringValue(strconv.FormatInt(rec.ID, 10))\n", idGo)
	if d.HasParent {
		fmt.Fprintf(&flat, "m.%s = flex.NullIntPtrToFramework(rec.%s)\n", d.ParentGo, pascalCase(pr.ParentJSON))
	}
	for _, f := range pr.Fields {
		mf, ok := byTF[f.TF]
		if !ok {
			return nil, fmt.Errorf("%s parent_read field %q is not a model attribute", pkg, f.TF)
		}
		expr := "rec." + pascalCase(f.From)
		if f.From == pr.Require {
			// Already the wrapper; reuse it rather than declaring the key twice.
			fmt.Fprintf(&flat, "m.%s = flex.NullIntPtrToFramework(rec.%s)\n", mf.GoName, d.RequireGo)
			d.UsesFlex = true
			continue
		}
		conv, err := kindConv(f.Kind, expr)
		if err != nil {
			return nil, fmt.Errorf("%s parent_read field %q: %w", pkg, f.TF, err)
		}
		if strings.HasPrefix(conv, "flex.") {
			d.UsesFlex = true
		}
		fmt.Fprintf(&flat, "m.%s = %s\n", mf.GoName, conv)
	}
	d.FlattenGo = flat.String()
	return d, nil
}
