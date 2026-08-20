package config

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Source abstracts reading the OpenAPI spec and the service packages so the
// derive/check/gen logic is unit-testable against a mock.
type Source interface {
	// Operations returns operationId -> {Method, Path} from the OpenAPI spec.
	Operations(specPath string) (map[string]Op, error)
	// ServiceOps returns, per service package under root, the SDK op names
	// invoked in each CRUD method of the resource and the Read of the data
	// source. Empty slices mean a stub (no SDK calls yet).
	ServiceOps(root string) ([]ServiceOps, error)
}

// Op is a resolved API operation.
type Op struct {
	Method string // GET, POST, PATCH, PUT, DELETE
	Path   string // e.g. /v3/label/{id}
}

// ServiceOps is the raw list of SDK op names a service's methods call.
type ServiceOps struct {
	Name                         string
	Create, Read, Update, Delete []string
	DataSourceRead               []string
}

type fileSource struct{}

// NewFileSource returns the production Source backed by the filesystem.
func NewFileSource() Source { return fileSource{} }

func (fileSource) Operations(specPath string) (map[string]Op, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	ops := make(map[string]Op)
	for path, methods := range doc.Paths {
		for method, op := range methods {
			switch strings.ToLower(method) {
			case "get", "post", "put", "patch", "delete":
				if op.OperationID != "" {
					ops[op.OperationID] = Op{Method: strings.ToUpper(method), Path: path}
				}
			}
		}
	}
	return ops, nil
}

func (fileSource) ServiceOps(root string) ([]ServiceOps, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []ServiceOps
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		so := ServiceOps{Name: name}
		if f := parseGo(filepath.Join(root, name, name+".go")); f != nil {
			so.Create = callsInMethod(f, "Create")
			so.Read = callsInMethod(f, "Read")
			so.Update = callsInMethod(f, "Update")
			so.Delete = callsInMethod(f, "Delete")
		}
		if f := parseGo(filepath.Join(root, name, name+"_data_source.go")); f != nil {
			so.DataSourceRead = callsInMethod(f, "Read")
		}
		out = append(out, so)
	}
	return out, nil
}

func parseGo(path string) *ast.File {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil
	}
	return f
}

// callsInMethod returns the selector names of every method call inside the named
// method (e.g. "conn.PostLabel(...)" -> "PostLabel"). Callers filter these
// against the operationId index to keep only real SDK ops.
func callsInMethod(f *ast.File, method string) []string {
	var names []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != method || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				names = append(names, sel.Sel.Name)
			}
			return true
		})
	}
	return names
}
