package crud

import (
	"fmt"
	"regexp"
)

// listedInDecl (kind: entity, archetype key listed_in) names the parent-scoped
// collection a record must also appear in to count as present. It is for a
// by-id GET that serves soft-deleted records while the collection filters them
// (#105): the writes stay by id, only the presence decision moves to the list.
type listedInDecl struct {
	Path        string `yaml:"path"`         // collection path with one {parent} placeholder
	ParentField string `yaml:"parent_field"` // Int64 model attribute holding the parent id
}

// listedInBind is the resolved declaration the templates render.
type listedInBind struct {
	PathFmt  string // Path with its placeholder as %d
	ParentGo string
	ParentTF string
}

var pathPlaceholder = regexp.MustCompile(`\{[^/{}]+\}`)

func bindListedIn(decl listedInDecl, model []ModelField) (*listedInBind, error) {
	if n := len(pathPlaceholder.FindAllString(decl.Path, -1)); n != 1 {
		return nil, fmt.Errorf("listed_in path %q must have exactly one {parent} placeholder, has %d", decl.Path, n)
	}
	for _, mf := range model {
		if mf.TFSDK != decl.ParentField {
			continue
		}
		if mf.Type != "types.Int64" {
			return nil, fmt.Errorf("listed_in parent_field %q is %s, want types.Int64", decl.ParentField, mf.Type)
		}
		return &listedInBind{
			PathFmt:  pathPlaceholder.ReplaceAllString(decl.Path, "%d"),
			ParentGo: mf.GoName,
			ParentTF: mf.TFSDK,
		}, nil
	}
	return nil, fmt.Errorf("listed_in parent_field %q is not in the model", decl.ParentField)
}
