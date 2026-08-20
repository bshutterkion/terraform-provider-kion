package kalign

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fileSource is the production Source: it parses Go source from disk.
type fileSource struct{}

// NewFileSource returns a Source that reads Go source files from the filesystem.
func NewFileSource() Source { return fileSource{} }

func (fileSource) SDKStructs(sdkFile string) (map[string][]SDKField, error) {
	f, err := parseGoFile(sdkFile)
	if err != nil {
		return nil, err
	}
	return structsFromFile(f), nil
}

func (fileSource) ServiceModels(root, only string) ([]ServiceModel, error) {
	var out []ServiceModel
	err := filepath.Walk(root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() || !strings.HasSuffix(path, "_schema_gen.go") {
			return nil
		}
		service := filepath.Base(filepath.Dir(path))
		if only != "" && service != only {
			return nil
		}
		f, perr := parseGoFile(path)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", path, perr)
		}
		if m := modelFromFile(f, service, path); m != nil {
			out = append(out, *m)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out, err
}

func (fileSource) FlexFuncs(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, perr := parseGoFile(filepath.Join(dir, e.Name()))
		if perr != nil {
			return nil, perr
		}
		addFlexFuncs(f, out)
	}
	return out, nil
}

func parseGoFile(path string) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), path, nil, 0)
}

// structsFromFile returns typeName -> ordered json-tagged fields for every
// struct type in f. Fields without a json tag are skipped.
func structsFromFile(f *ast.File) map[string][]SDKField {
	out := map[string][]SDKField{}
	forEachStruct(f, func(name string, st *ast.StructType) {
		var fields []SDKField
		for _, fld := range st.Fields.List {
			if len(fld.Names) == 0 {
				continue
			}
			j := tagValue(fld.Tag, "json")
			if j == "" {
				continue
			}
			fields = append(fields, SDKField{GoName: fld.Names[0].Name, JSON: j, GoType: typeString(fld.Type)})
		}
		if len(fields) > 0 {
			out[name] = fields
		}
	})
	return out
}

// modelFromFile returns the first *Model struct in f as a ServiceModel, or nil.
func modelFromFile(f *ast.File, service, path string) *ServiceModel {
	var found *ServiceModel
	forEachStruct(f, func(name string, st *ast.StructType) {
		if found != nil || !strings.HasSuffix(name, "Model") {
			return
		}
		var fields []ModelField
		for _, fld := range st.Fields.List {
			if len(fld.Names) == 0 {
				continue
			}
			t := tagValue(fld.Tag, "tfsdk")
			if t == "" {
				continue
			}
			fields = append(fields, ModelField{GoName: fld.Names[0].Name, TFSDK: t, TFType: typeString(fld.Type)})
		}
		found = &ServiceModel{Service: service, Name: name, File: path, Fields: fields}
	})
	return found
}

// addFlexFuncs records every top-level (non-method) function name in f.
func addFlexFuncs(f *ast.File, into map[string]bool) {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			into[fn.Name.Name] = true
		}
	}
}

// forEachStruct calls fn for each named struct type declared in f.
func forEachStruct(f *ast.File, fn func(name string, st *ast.StructType)) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				fn(ts.Name.Name, st)
			}
		}
	}
}
