package crud

import (
	_ "embed"
	"fmt"
	"path/filepath"
)

//go:embed blended.gtpl
var blendedTmpl string

// blendedKind is the crud_archetypes.yaml kind for a resource whose CRUD is
// split per-op between the typed public SDK (/v3,/v4,/beta) and raw private
// HTTP (/v1,/v2). The public ops (always create; usually update/delete) are
// generated exactly like the entity archetype; the private ops (always the
// by-id read; sometimes update or delete) are generated over conns.Raw*.
const blendedKind = "blended"

// rawReadField is one flat scalar field of a blended resource's raw-read wire
// struct. Reuses rawField's expressions (ToExpr for flatten, FromExpr for a raw
// write body).
type rawReadData struct {
	Method string     // "RawGet"
	Path   string     // "/v1/app-role/{id}"
	IDGo   string     // model id field GoName
	Fields []rawField // flat scalar wire fields (excludes id) — flat reads only
	// Nested reads (declared read_shape): the generator emits WireStructGo and a
	// flatten(ctx, w, m) diag.Diagnostics body rather than the flat wire machinery.
	Nested       bool
	WireStructGo string // the full `type <pkg>Wire struct {…}` text
	FlattenGo    string // the flatten function body text
	// UsesAttr reports whether the flatten body constructs a nested Value and
	// therefore needs the framework attr package. A read_shape of only scalars
	// assigns plain types.* values and must not import it.
	UsesAttr bool
}

// rawWriteData is a blended resource's raw update or delete op.
type rawWriteData struct {
	Method  string // "RawPatch" | "RawDelete"
	Path    string // "/v2/project-note/{id}"
	HasBody bool   // update marshals the model into the wire body; delete does not
}

// isRawOp reports whether an op is served over the private API (rendered raw).
func isRawOp(op rawOp) bool {
	return op.Spec == "private" && op.Path != ""
}

// generateBlended resolves and writes a blended resource (<name>.go +
// service_package.go, resource-only like the raw archetype).
func (g *generator) generateBlended(dir, name string, ops resOps, idx sdkIndex, pe rawResourceOps, model []ModelField, gated, force bool) (int, error) {
	schemaGen := filepath.Join(dir, name+"_schema_gen.go")
	d, err := g.resolveBlended(name, ops, idx, pe, model, gated, schemaGen)
	if err != nil {
		return 0, err
	}
	resourceGo, err := execGoTemplate("blended", blendedTmpl, d, name+".go")
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

// resolveBlended builds the entityData for a blended resource: the typed public
// ops go through the same resolution + binds as the entity archetype (via
// buildEntityData over a partial ResourceModel), and the private ops are
// layered on as raw read/update/delete.
func (g *generator) resolveBlended(name string, ops resOps, idx sdkIndex, pe rawResourceOps, model []ModelField, gated bool, schemaGen string) (entityData, error) {
	if pe.Read.Path == "" || !isRawOp(pe.Read) {
		return entityData{}, fmt.Errorf("%s: blended archetype requires a private read in private_endpoints.yaml", name)
	}

	pascal := pascalCase(name)
	rm := ResourceModel{Name: name, Pascal: pascal, Model: pascal + "Model", Gated: gated}

	// Typed public create (required).
	create, err := resolveOp("create", ops.Create, idx)
	if err != nil {
		return entityData{}, fmt.Errorf("%s create: %w", name, err)
	}
	if create == nil {
		return entityData{}, fmt.Errorf("%s: blended archetype requires a typed public create", name)
	}
	rm.Create = *create

	// Typed public update/delete — only when that op is NOT private.
	if !isRawOp(pe.Update) {
		if rm.Update, err = resolveOp("update", ops.Update, idx); err != nil {
			return entityData{}, fmt.Errorf("%s update: %w", name, err)
		}
	}
	if !isRawOp(pe.Delete) {
		if rm.Delete, err = resolveOp("delete", ops.Delete, idx); err != nil {
			return entityData{}, fmt.Errorf("%s delete: %w", name, err)
		}
	}

	for _, mf := range model {
		if mf.TFSDK == "id" {
			rm.IDField = mf
			continue
		}
		rm.Fields = append(rm.Fields, mf)
	}
	if rm.IDField.TFSDK == "" {
		return entityData{}, fmt.Errorf("%s: generated model %s has no tfsdk:%q field", name, rm.Model, "id")
	}

	// Nested create/update expand: a blended create/update body may still carry
	// nested objects (permission_scheme roles, billing_source aws_connection).
	byTF := map[string]ModelField{}
	for _, mf := range model {
		byTF[mf.TFSDK] = mf
	}
	if rm.CreateNested, err = resolveNested(g.src, schemaGen, rm.Create.Body, byTF, idx, pe.NoGuard); err != nil {
		return entityData{}, fmt.Errorf("%s create nested: %w", name, err)
	}
	if rm.Update != nil {
		if rm.UpdateNested, err = resolveNested(g.src, schemaGen, rm.Update.Body, byTF, idx, pe.NoGuard); err != nil {
			return entityData{}, fmt.Errorf("%s update nested: %w", name, err)
		}
	}

	d, err := buildEntityData(rm)
	if err != nil {
		return entityData{}, fmt.Errorf("%s: %w", name, err)
	}
	d.Blended = true

	// buildEntityData derives IDParamType from the read/delete params; a blended
	// resource has no typed read, and delete may be raw, so source the typed
	// id-param Go type from whichever typed op has a params struct.
	if d.IDParamType == "" {
		d.IDParamType = "int64"
		for _, op := range []*OpModel{rm.Update, rm.Delete} {
			if op == nil {
				continue
			}
			if _, t, e := idParamName(op.Params); e == nil {
				d.IDParamType = t
				break
			}
		}
	}

	// Raw private read: a declared nested shape (read_shape) when the wire isn't
	// a flat 1:1 of the model scalars, else the flat wire from the model.
	readMethod, err := rawVerb(pe.Read.Method)
	if err != nil {
		return entityData{}, fmt.Errorf("%s raw read: %w", name, err)
	}
	if pe.ReadShape != nil {
		wireGo, err := buildWireStruct(name, *pe.ReadShape)
		if err != nil {
			return entityData{}, fmt.Errorf("%s read_shape wire: %w", name, err)
		}
		flattenGo, err := buildNestedFlatten(*pe.ReadShape, byTF)
		if err != nil {
			return entityData{}, fmt.Errorf("%s read_shape flatten: %w", name, err)
		}
		d.RawRead = &rawReadData{Method: readMethod, Path: pe.Read.Path, IDGo: rm.IDField.GoName, Nested: true, WireStructGo: wireGo, FlattenGo: flattenGo,
			UsesAttr: len(pe.ReadShape.Objects) > 0 || pe.ReadShape.Explode != nil}
	} else {
		fields, idGo, ferr := rawModelFields(model)
		if ferr != nil {
			return entityData{}, fmt.Errorf("%s raw read: %w", name, ferr)
		}
		d.RawRead = &rawReadData{Method: readMethod, Path: pe.Read.Path, IDGo: idGo, Fields: fields}
	}

	if isRawOp(pe.Update) {
		m, err := rawVerb(pe.Update.Method)
		if err != nil {
			return entityData{}, fmt.Errorf("%s raw update: %w", name, err)
		}
		d.RawUpdate = &rawWriteData{Method: m, Path: pe.Update.Path, HasBody: true}
	}
	if isRawOp(pe.Delete) {
		m, err := rawVerb(pe.Delete.Method)
		if err != nil {
			return entityData{}, fmt.Errorf("%s raw delete: %w", name, err)
		}
		d.RawDelete = &rawWriteData{Method: m, Path: pe.Delete.Path, HasBody: false}
	}

	return d, nil
}
