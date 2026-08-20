package crud

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed servicepackage_noread.gtpl
var servicePackageNoReadTmpl string

// noReadKind is the crud_archetypes.yaml kind for a resource with no
// single-record GET endpoint (create → id, no-op read keeping state, delete).
const noReadKind = "no_read"

// resolveNoRead assembles a ResourceModel for a no-read resource: create,
// (optional) update, and delete ops, with Read left empty so buildEntityData
// renders the no-read template.
func resolveNoRead(name string, ops resOps, idx sdkIndex, model []ModelField) (ResourceModel, error) {
	pascal := pascalCase(name)
	rm := ResourceModel{Name: name, Pascal: pascal, Model: pascal + "Model"}

	create, err := resolveOp("create", ops.Create, idx)
	if err != nil {
		return rm, err
	}
	if create == nil {
		return rm, fmt.Errorf("%s: no-read archetype requires a create op", name)
	}
	rm.Create = *create

	if rm.Update, err = resolveOp("update", ops.Update, idx); err != nil {
		return rm, err
	}
	if rm.Delete, err = resolveOp("delete", ops.Delete, idx); err != nil {
		return rm, err
	}

	for _, mf := range model {
		if mf.TFSDK == "id" {
			rm.IDField = mf
			continue
		}
		rm.Fields = append(rm.Fields, mf)
	}
	if rm.IDField.TFSDK == "" {
		return rm, fmt.Errorf("%s: generated model %s has no tfsdk:%q field — add one via codegen/schema_overrides.yaml for no-read resources", name, rm.Model, "id")
	}
	return rm, nil
}

// generateNoRead writes the files for a no-read resource: the resource, a stub
// sweeper, and a resource-only service_package.go (no data source, since there
// is no read endpoint to back one).
func (g *generator) generateNoRead(dir, name string, ops resOps, idx sdkIndex, model []ModelField, gated, force bool) (int, error) {
	rm, err := resolveNoRead(name, ops, idx, model)
	if err != nil {
		return 0, err
	}
	rm.Gated = gated

	resourceGo, err := renderEntity(rm)
	if err != nil {
		return 0, err
	}
	sweepGo, err := renderSweep(rm)
	if err != nil {
		return 0, err
	}
	pkgGo, err := execGoTemplate("servicepackage_noread", servicePackageNoReadTmpl, struct{ Pkg, Pascal string }{name, rm.Pascal}, "service_package.go")
	if err != nil {
		return 0, err
	}

	files := []genFile{
		{filepath.Join(dir, name+".go"), resourceGo},
		{filepath.Join(dir, "sweep.go"), sweepGo},
		{filepath.Join(dir, "service_package.go"), pkgGo},
	}
	fmt.Fprintf(os.Stderr, "kgen crud: %s — no-read archetype; no data source emitted (no read endpoint)\n", name)
	for _, f := range files {
		if err := g.writeFile(f.path, f.data, force); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

// dataSourceCompanionCtor returns the data-source constructor for a resource
// whose archetype emits no read, but which nonetheless ships a hand-authored
// data source as a companion file (see companionsByName).
//
// Without this the noread service package always emitted `return nil`, so those
// data sources were generated, compiled, tested — and never registered, leaving
// them unreachable from a practitioner's config. Registering them by hand did
// not survive, since the next `kgen crud` regenerated the file.
func dataSourceCompanionCtor(name, pascal string) string {
	for _, f := range companionsByName[name] {
		if strings.HasSuffix(f.outName, "_data_source.go") {
			return "New" + pascal + "DataSource"
		}
	}
	return ""
}
