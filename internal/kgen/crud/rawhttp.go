package crud

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed rawhttp.gtpl
var rawHTTPTmpl string

// rawKind is the crud_archetypes.yaml kind for a resource whose read (and
// sometimes other ops) hit private backend endpoints the generated SDK lacks;
// CRUD is done over raw HTTP with JSON marshaled from the model fields.
const rawKind = "raw_http"

// rawOp is one endpoint from codegen/private_endpoints.yaml.
type rawOp struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
	Spec   string `yaml:"spec"` // "public" | "private" — blended archetype renders private ops raw
}

type rawResourceOps struct {
	Create    rawOp      `yaml:"create"`
	Read      rawOp      `yaml:"read"`
	Update    rawOp      `yaml:"update"`
	Delete    rawOp      `yaml:"delete"`
	ReadShape *readShape `yaml:"read_shape"` // blended: declared nested private-read shape
	// NoGuard disables the "multiple optional nested objects are alternatives"
	// heuristic (see objBind.Guard). Set it for a body whose optional nested
	// objects are genuine companions, where sending only the configured one
	// would drop the others.
	NoGuard bool `yaml:"no_guard"`
}

// loadPrivateEndpoints reads codegen/private_endpoints.yaml. Missing file → empty.
func loadPrivateEndpoints(path string) (map[string]rawResourceOps, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]rawResourceOps{}, nil
		}
		return nil, fmt.Errorf("reading private endpoints %s: %w", path, err)
	}
	var f struct {
		Resources map[string]rawResourceOps `yaml:"resources"`
	}
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parsing private endpoints %s: %w", path, err)
	}
	return f.Resources, nil
}

// rawField binds one model field to its JSON wire representation.
type rawField struct {
	ModelGo  string // "Name"
	JSON     string // "name"
	WireType string // "string" | "int64" | "bool"
	FromExpr string // "plan.Name.ValueString()"
	ToExpr   string // "types.StringValue(w.Name)"
}

// rawData is the raw-http template payload.
type rawData struct {
	Pkg, Pascal, Model, ResConst, ResName string
	Gated                                 bool
	TypeName                              string
	IDGo                                  string

	CreateMethod, CreatePath string
	ReadMethod, ReadPath     string
	HasUpdate                bool
	UpdateMethod, UpdatePath string
	HasDelete                bool
	DeleteMethod, DeletePath string

	Fields []rawField // non-id
}

// rawVerb maps an HTTP method to the conns raw verb name.
func rawVerb(method string) (string, error) {
	switch method {
	case "GET":
		return "RawGet", nil
	case "POST":
		return "RawPost", nil
	case "PATCH":
		return "RawPatch", nil
	case "PUT":
		return "RawPut", nil
	case "DELETE":
		return "RawDelete", nil
	}
	return "", fmt.Errorf("unsupported raw HTTP method %q", method)
}

func resolveRaw(name string, ops rawResourceOps, model []ModelField) (rawData, error) {
	pascal := pascalCase(name)
	d := rawData{
		Pkg:      name,
		Pascal:   pascal,
		Model:    pascal + "Model",
		ResConst: "ResName" + pascal,
		ResName:  pascal + " Resource",
		TypeName: "kion_" + name,
	}
	if ops.Create.Path == "" || ops.Read.Path == "" {
		return d, fmt.Errorf("%s: raw_http archetype requires create + read in private_endpoints.yaml", name)
	}
	var err error
	if d.CreateMethod, err = rawVerb(ops.Create.Method); err != nil {
		return d, err
	}
	d.CreatePath = ops.Create.Path
	if d.ReadMethod, err = rawVerb(ops.Read.Method); err != nil {
		return d, err
	}
	d.ReadPath = ops.Read.Path
	if ops.Update.Path != "" {
		d.HasUpdate = true
		if d.UpdateMethod, err = rawVerb(ops.Update.Method); err != nil {
			return d, err
		}
		d.UpdatePath = ops.Update.Path
	}
	if ops.Delete.Path != "" {
		d.HasDelete = true
		if d.DeleteMethod, err = rawVerb(ops.Delete.Method); err != nil {
			return d, err
		}
		d.DeletePath = ops.Delete.Path
	}

	fields, idGo, err := rawModelFields(model)
	if err != nil {
		return d, fmt.Errorf("%s: %w", name, err)
	}
	d.Fields, d.IDGo = fields, idGo
	if d.IDGo == "" {
		return d, fmt.Errorf("%s: model %s has no tfsdk:%q field", name, d.Model, "id")
	}
	return d, nil
}

// rawModelFields maps a model's scalar fields to their JSON wire representation
// (String/Int64/Bool) and returns the id field's Go name. Shared by the fully
// raw archetype and the blended archetype's raw ops. It refuses any non-scalar
// field (nested reads declare their own shape).
func rawModelFields(model []ModelField) (fields []rawField, idGo string, err error) {
	for _, mf := range model {
		if mf.TFSDK == "id" {
			idGo = mf.GoName
			continue
		}
		f := rawField{ModelGo: mf.GoName, JSON: mf.TFSDK}
		switch mf.Type {
		case "types.String":
			f.WireType, f.FromExpr, f.ToExpr = "string", "plan."+mf.GoName+".ValueString()", "types.StringValue(w."+mf.GoName+")"
		case "types.Int64":
			f.WireType, f.FromExpr, f.ToExpr = "int64", "plan."+mf.GoName+".ValueInt64()", "types.Int64Value(w."+mf.GoName+")"
		case "types.Bool":
			f.WireType, f.FromExpr, f.ToExpr = "bool", "plan."+mf.GoName+".ValueBool()", "types.BoolValue(w."+mf.GoName+")"
		default:
			return nil, "", fmt.Errorf("raw field %q has unsupported type %q (only scalar String/Int64/Bool)", mf.TFSDK, mf.Type)
		}
		fields = append(fields, f)
	}
	return fields, idGo, nil
}

// generateRaw writes a raw-http resource + resource-only service_package (raw
// resources don't emit a data source).
func (g *generator) generateRaw(dir, name string, ops rawResourceOps, model []ModelField, gated, force bool) (int, error) {
	d, err := resolveRaw(name, ops, model)
	if err != nil {
		return 0, err
	}
	d.Gated = gated
	resourceGo, err := execGoTemplate("rawhttp", rawHTTPTmpl, d, name+".go")
	if err != nil {
		return 0, err
	}
	pkgGo, err := execGoTemplate("servicepackage_noread", servicePackageNoReadTmpl, struct{ Pkg, Pascal, DataSourceCtor string }{name, d.Pascal, dataSourceCompanionCtor(name, d.Pascal)}, "service_package.go")
	if err != nil {
		return 0, err
	}
	files := []genFile{
		{filepath.Join(dir, name+".go"), resourceGo},
		{filepath.Join(dir, "service_package.go"), pkgGo},
	}
	for _, f := range files {
		if err := g.writeFile(f.path, f.data, force); err != nil {
			return 0, err
		}
	}
	return 1, nil
}
