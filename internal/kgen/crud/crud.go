// Package crud generates CRUD methods, data sources, acceptance tests, and
// sweepers for entity-archetype service packages by AST-introspecting the
// ogen SDK. It imports neither the SDK nor internal/service, so it runs
// mid-development even when the provider does not compile.
package crud

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"terraform-provider-kion/internal/kgen/kfs"
	"terraform-provider-kion/internal/kgen/migrate"
)

// Options configures a crud generation run.
type Options struct {
	ProjectRoot     string // resolved via findProjectRoot() when empty
	Config          string // codegen/generator_config.yaml
	ConfigOverrides string // codegen/config_overrides.yaml (reserved; the merged config is read directly)
	SDKDir          string // ../kion-sdk-go
	SDKVersion      string // "v3_16"
	CrudOverrides   string // codegen/crud_overrides.yaml
	Archetypes      string // codegen/crud_archetypes.yaml (compound-key/parent-read declarations)
	TestValues      string // codegen/test_values.yaml
	VersionSupport  string // codegen/version_support.yaml
	Force           bool
	OnlyResource    string // "" = all
	// Strict fails the run when any data source was downgraded from
	// list+filter to id-only. Off by default because a handful of resources
	// legitimately have no callable collection endpoint; on in a gate that wants
	// the set frozen.
	Strict bool
}

// downgrade records a data source that fell back from the list+filter shape to
// id-only. Losing the `filter` block is a silent, config-breaking regression for
// anyone migrating from the SDKv2 provider, so these are reported loudly and in
// aggregate at the end of a run rather than as a lone line mid-stream.
type downgrade struct {
	Resource string
	Reason   string
}

type generator struct {
	fs          kfs.FS
	src         Source
	privEnds    map[string]rawResourceOps    // codegen/private_endpoints.yaml (raw_http archetype)
	memberships map[string]membershipConfig  // codegen/memberships.yaml (owner sync on update)
	upgrades    map[string]migrate.Transform // codegen/state_upgrades.yaml (state upgraders)
	oldSchema   map[string]migrate.Resource  // codegen/schema_snapshots/old.json (lazy)
	newSchema   map[string]migrate.Resource  // codegen/schema_snapshots/new.json (lazy)
	root        string
	// fieldPolicy is codegen/unexposed_fields.yaml: the response fields a list
	// data source must withhold. Loaded once per run alongside the other
	// codegen inputs above.
	fieldPolicy FieldPolicy
	downgrades  []downgrade // data sources that lost their filter block this run
	// dataSources is the generator_config `data_sources` op-set, needed to reach
	// a resource OTHER than the one being generated: a parent-scoped sweeper
	// enumerates its parent's collection.
	dataSources map[string]dsOps
	unswept     []downgrade // resources that got no sweeper this run, with the reason
}

// Generate is the stage entry point.
func Generate(opts Options) (int, error) {
	g := &generator{fs: kfs.OS{}, src: NewFileSource()}
	return g.generate(opts)
}

func (g *generator) generate(opts Options) (int, error) {
	root := opts.ProjectRoot
	if root == "" {
		r, err := findProjectRoot()
		if err != nil {
			return 0, err
		}
		root = r
	}

	policy, err := LoadFieldPolicy(root)
	if err != nil {
		return 0, err
	}
	g.fieldPolicy = policy

	cfgPath := opts.Config
	if cfgPath == "" {
		cfgPath = filepath.Join("codegen", "generator_config.yaml")
	}
	cfgPath = resolveAgainst(root, cfgPath)

	resources, dataSources, flagged, err := loadConfig(cfgPath)
	if err != nil {
		return 0, err
	}
	g.dataSources = dataSources

	sdkDir := opts.SDKDir
	if sdkDir == "" {
		sdkDir = "../kion-sdk-go"
	}
	sdkDir = resolveAgainst(root, sdkDir)

	idx, err := buildIndex(g.src, sdkDir, opts.SDKVersion)
	if err != nil {
		return 0, err
	}

	tvPath := opts.TestValues
	if tvPath == "" {
		tvPath = filepath.Join("codegen", "test_values.yaml")
	}
	tvPath = resolveAgainst(root, tvPath)

	vsPath := opts.VersionSupport
	if vsPath == "" {
		vsPath = filepath.Join("codegen", "version_support.yaml")
	}
	gated, err := loadVersionSupport(resolveAgainst(root, vsPath))
	if err != nil {
		return 0, err
	}

	archPath := opts.Archetypes
	if archPath == "" {
		archPath = filepath.Join("codegen", "crud_archetypes.yaml")
	}
	archetypes, err := loadArchetypes(resolveAgainst(root, archPath))
	if err != nil {
		return 0, err
	}

	g.privEnds, err = loadPrivateEndpoints(resolveAgainst(root, filepath.Join("codegen", "private_endpoints.yaml")))
	if err != nil {
		return 0, err
	}

	g.memberships, err = loadMemberships(resolveAgainst(root, filepath.Join("codegen", "memberships.yaml")))
	if err != nil {
		return 0, err
	}

	// State upgraders (old SDKv2 → new schema). Optional file; absent = no migration.
	g.root = root
	if ups, uerr := migrate.LoadUpgrades(resolveAgainst(root, filepath.Join("codegen", "state_upgrades.yaml"))); uerr == nil {
		g.upgrades = ups
	}

	// Process every configured resource plus any bespoke-archetype resource that
	// is declared only in crud_archetypes.yaml (bespoke resources carry their
	// schema inline in a template, so they are intentionally absent from the
	// generator_config `resources` list that drives the schema stage).
	names := map[string]bool{}
	for name := range resources {
		names[name] = true
	}
	for name, a := range archetypes {
		if isBespokeKind(a.Kind) {
			names[name] = true
		}
	}

	written := 0
	for _, name := range sortedKeys(names) {
		if opts.OnlyResource != "" && name != opts.OnlyResource {
			continue
		}
		if flagged[name] {
			fmt.Fprintf(os.Stderr, "kgen crud: skip %s, flagged for review in generator_config.yaml\n", name)
			continue
		}
		var arch *archetype
		if a, ok := archetypes[name]; ok {
			arch = &a
		}
		n, err := g.generateResource(root, name, resources[name], dataSources[name], idx, tvPath, gated[name], arch, opts.Force)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kgen crud: skip %s: %v\n", name, err)
			continue
		}
		// Emit the state upgrader for any resource that migrates old SDKv2 state.
		if err := g.emitUpgrade(name, opts.Force); err != nil {
			fmt.Fprintf(os.Stderr, "kgen crud: %s upgrader: %v\n", name, err)
		}
		written += n
	}
	g.reportUnswept()
	return written, g.reportDowngrades(opts.Strict)
}

// reportUnswept prints the aggregated no-sweeper report. Each entry is a
// resource whose orphaned test-acc records `make sweep` will NOT clean up. It is
// not an error: several are structurally unsweepable (no delete endpoint at
// all). It is printed so the set is visible rather than silently assumed empty.
func (g *generator) reportUnswept() {
	if len(g.unswept) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n-- kgen crud: %d resource(s) got NO sweeper; `make sweep` leaves their\n", len(g.unswept))
	fmt.Fprintf(os.Stderr, "-- orphaned test-acc records behind. Each sweep.go says the same.\n")
	for _, u := range g.unswept {
		fmt.Fprintf(os.Stderr, "--   kion_%s: %s\n", u.Resource, u.Reason)
	}
	fmt.Fprint(os.Stderr, "\n")
}

// reportDowngrades prints the aggregated downgrade report. Every entry means a
// data source that SHOULD accept `filter` blocks (the config maps a collection
// read for it) instead demands a required `id`, which silently breaks
// configurations carried over from the SDKv2 provider. Under strict it is an
// error, so the set cannot grow unnoticed.
func (g *generator) reportDowngrades(strict bool) error {
	if len(g.downgrades) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "\n!! kgen crud: %d DATA SOURCE(S) DOWNGRADED to id-only. Each lost its `filter` block\n", len(g.downgrades))
	fmt.Fprintf(os.Stderr, "!! and now REQUIRES `id`, which breaks configurations written for the old provider.\n")
	for _, d := range g.downgrades {
		fmt.Fprintf(os.Stderr, "!!   kion_%s: %s\n", d.Resource, d.Reason)
	}
	fmt.Fprintf(os.Stderr, "!! Fix the generator (internal/kgen/crud) or accept each case deliberately.\n\n")
	if strict {
		return fmt.Errorf("%d data source(s) downgraded to id-only (--strict)", len(g.downgrades))
	}
	return nil
}

// generateResource resolves one resource and writes its generated files:
// <name>.go, <name>_data_source.go, and (when test values exist) the
// acceptance-test files.
func (g *generator) generateResource(root, name string, ops resOps, ds dsOps, idx sdkIndex, tvPath string, gated bool, arch *archetype, force bool) (int, error) {
	pascal := pascalCase(name)
	dir := filepath.Join(root, "internal", "service", name)

	// Bespoke archetypes emit verbatim resource/data-source bodies and need
	// neither the schema_gen model nor the resolved op-set, so dispatch before
	// reading either (these resources have no *Model in a schema_gen file).
	if arch != nil {
		switch arch.Kind {
		case singletonKind:
			return g.generateSingleton(dir, name, force)
		case cvOverrideKind:
			return g.generateCVOverride(dir, name, force)
		case cloudAccountKind:
			return g.generateCloudAccount(dir, name, force)
		case datasourceOnlyKind:
			return g.generateDatasourceOnly(dir, name, force)
		}
	}

	model, err := g.src.ModelFields(filepath.Join(dir, name+"_schema_gen.go"), pascal+"Model")
	if err != nil {
		return 0, err
	}

	var entityArch *archetype
	if arch != nil {
		switch arch.Kind {
		case noReadKind:
			return g.generateNoRead(dir, name, ops, idx, model, gated, force)
		case assocKind:
			return g.generateAssoc(dir, name, ops, idx, *arch, model, gated, force)
		case rawKind:
			pe, ok := g.privEnds[name]
			if !ok {
				return 0, fmt.Errorf("%s: raw_http archetype but no codegen/private_endpoints.yaml entry", name)
			}
			return g.generateRaw(dir, name, pe, model, gated, force)
		case blendedKind:
			pe, ok := g.privEnds[name]
			if !ok {
				return 0, fmt.Errorf("%s: blended archetype but no codegen/private_endpoints.yaml entry", name)
			}
			return g.generateBlended(dir, name, ops, idx, pe, model, gated, force)
		case parentListKind:
			return g.generateParentList(dir, name, ops, idx, *arch, model, gated, force)
		case entityKind:
			entityArch = arch // normal entity path, with archetype tweaks applied below
		default:
			return g.generateCompound(dir, name, ops, idx, *arch, model, gated, force)
		}
	}

	rm, err := resolveResource(name, ops, ds, idx, model)
	if err != nil {
		return 0, err
	}
	rm.Gated = gated
	if _, ok := g.upgrades["kion_"+name]; ok {
		rm.SchemaVersion = 1
	}
	if entityArch != nil {
		rm.DeleteRecordParam = entityArch.DeleteRecordParam
		rm.DeleteExtraParam = entityArch.DeleteExtraParam
		rm.DeleteExtraField = entityArch.DeleteExtraField
		if entityArch.SweepParent != "" {
			if err := g.bindSweepParent(&rm, ds, idx, *entityArch); err != nil {
				fmt.Fprintf(os.Stderr, "kgen crud: %s: %v\n", name, err)
			}
		}
	}

	// Resolve nested-object body/response fields (needs the schema_gen Value types).
	schemaGen := filepath.Join(dir, name+"_schema_gen.go")
	byTF := map[string]ModelField{}
	for _, mf := range model {
		byTF[mf.TFSDK] = mf
	}

	// A model that nests the whole record under one object attribute exposes none
	// of the record's fields at the top level; promote them so the data source and
	// sweeper can still project the record.
	rm.RecordSubFields = recordSubFields(g.src, schemaGen, rm.Read.RespWrapperGo, byTF)

	// Create-under-parent: the create op's id param is the parent id from a model
	// attribute (not a literal discriminator). Overrides the literal create-param
	// path resolved above.
	if entityArch != nil && entityArch.CreateParentParam != "" {
		mf, ok := byTF[entityArch.CreateParentField]
		if !ok {
			return 0, fmt.Errorf("%s: create_parent_field %q not in model", name, entityArch.CreateParentField)
		}
		rm.CreateParentParam = entityArch.CreateParentParam
		rm.CreateParentField = mf.GoName
		if rm.Create.Params != nil {
			for _, f := range rm.Create.Params.Fields {
				if f.GoName == entityArch.CreateParentParam && f.Type != "int64" && f.Type != "" {
					rm.CreateParentCast = f.Type
				}
			}
		}
	}
	if rm.CreateNested, err = resolveNested(g.src, schemaGen, rm.Create.Body, byTF, idx, false); err != nil {
		return 0, fmt.Errorf("%s create nested: %w", name, err)
	}
	if rm.Update != nil {
		if rm.UpdateNested, err = resolveNested(g.src, schemaGen, rm.Update.Body, byTF, idx, false); err != nil {
			return 0, fmt.Errorf("%s update nested: %w", name, err)
		}
	}
	readPrefix := "v.Data.Value."
	if rm.Read.RespDataPtr {
		readPrefix = "v.Data."
	}
	if rm.ReadNested, err = resolveNestedFlatten(g.src, schemaGen, rm.Read.RespFields, byTF, idx, readPrefix); err != nil {
		return 0, fmt.Errorf("%s read nested: %w", name, err)
	}

	// Owner association synced on Update via paired add/remove endpoints.
	if mc, ok := g.memberships[name]; ok {
		if mc.Owners != nil {
			if rm.Owners, err = resolveOwnerMembership(*mc.Owners, byTF); err != nil {
				return 0, fmt.Errorf("%s owners: %w", name, err)
			}
		}
		for i := range mc.Associations {
			a, aerr := resolveAssocMembership(mc.Associations[i], byTF, idx)
			if aerr != nil {
				return 0, fmt.Errorf("%s associations: %w", name, aerr)
			}
			rm.Assocs = append(rm.Assocs, a)
		}
		for i := range mc.SliceMembers {
			s, serr := resolveSliceMember(mc.SliceMembers[i], byTF)
			if serr != nil {
				return 0, fmt.Errorf("%s slice_members: %w", name, serr)
			}
			rm.SliceMembers = append(rm.SliceMembers, s)
		}
	}

	resourceGo, err := renderEntity(rm)
	if err != nil {
		return 0, err
	}
	dataSourceGo, dsDowngrade, err := renderDataSource(rm, g.fieldPolicy)
	if err != nil {
		return 0, err
	}
	if dsDowngrade != "" {
		g.downgrades = append(g.downgrades, downgrade{Resource: name, Reason: dsDowngrade})
	}
	sweepGo, sweepReason, err := renderSweep(rm)
	if err != nil {
		return 0, err
	}
	if sweepReason != "" {
		g.unswept = append(g.unswept, downgrade{Resource: name, Reason: sweepReason})
	}

	files := []genFile{
		{filepath.Join(dir, name+".go"), resourceGo},
		{filepath.Join(dir, name+"_data_source.go"), dataSourceGo},
		{filepath.Join(dir, "sweep.go"), sweepGo},
	}

	tv, hasTV, err := loadTestValues(tvPath, name)
	if err != nil {
		return 0, err
	}
	if hasTV {
		resTest, err := renderResourceTest(rm, tv)
		if err != nil {
			return 0, err
		}
		dsTest, err := renderDataSourceTest(rm, tv)
		if err != nil {
			return 0, err
		}
		files = append(files,
			genFile{filepath.Join(dir, name+"_test.go"), resTest},
			genFile{filepath.Join(dir, name+"_data_source_test.go"), dsTest},
		)
	} else {
		fmt.Fprintf(os.Stderr, "kgen crud: %s: no test_values entry; skipping acceptance tests\n", name)
	}

	for _, f := range files {
		if err := g.writeFile(f.path, f.data, force); err != nil {
			return 0, err
		}
	}
	// Emit any hand-authored companion files kept beside this resource (e.g. the
	// aws_cloudformation_template / aws_iam_policy alias registrations).
	if err := g.emitCompanions(dir, name, force); err != nil {
		return 0, err
	}
	return 1, nil
}

// generateCompound writes the files for a compound-key/parent-read resource.
// Acceptance tests are deferred (they need a parent FK fixture), so only the
// resource, data source, and sweeper stub are emitted.
func (g *generator) generateCompound(dir, name string, ops resOps, idx sdkIndex, arch archetype, model []ModelField, gated, force bool) (int, error) {
	if arch.Kind != compoundKindParentRead {
		return 0, fmt.Errorf("%s: unknown archetype kind %q", name, arch.Kind)
	}
	d, err := resolveCompound(name, ops, idx, arch, model, gated)
	if err != nil {
		return 0, err
	}
	resourceGo, err := renderCompoundEntity(d)
	if err != nil {
		return 0, err
	}
	dataSourceGo, err := renderCompoundDataSource(d)
	if err != nil {
		return 0, err
	}
	// A compound-key sub-resource has no independent collection: records are
	// enumerated per parent, which needs the FK-fixture sweeper follow-up.
	sweepReason := "a compound-key sub-resource has no independent collection endpoint"
	sweepGo, err := renderNoSweep(name, d.TypeName, sweepReason)
	if err != nil {
		return 0, err
	}
	g.unswept = append(g.unswept, downgrade{Resource: name, Reason: sweepReason})
	files := []genFile{
		{filepath.Join(dir, name+".go"), resourceGo},
		{filepath.Join(dir, name+"_data_source.go"), dataSourceGo},
		{filepath.Join(dir, "sweep.go"), sweepGo},
	}
	fmt.Fprintf(os.Stderr, "kgen crud: %s: compound archetype; acceptance tests skipped (need a parent FK fixture)\n", name)
	for _, f := range files {
		if err := g.writeFile(f.path, f.data, force); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

// genFile is a rendered file destined for a service package.
type genFile struct {
	path string
	data []byte
}

// emitUpgrade writes <name>_upgrade_gen.go for a resource that migrates old
// SDKv2 state (has a codegen/state_upgrades.yaml entry). No-op otherwise. The
// resource's schema.Version bump is emitted separately by the archetype template
// (gated on ResourceModel.SchemaVersion).
func (g *generator) emitUpgrade(name string, force bool) error {
	tfType := "kion_" + name
	t, ok := g.upgrades[tfType]
	if !ok {
		return nil
	}
	if g.oldSchema == nil {
		old, err := migrate.LoadSchema(resolveAgainst(g.root, filepath.Join("codegen", "schema_snapshots", "old.json")))
		if err != nil {
			return err
		}
		nw, err := migrate.LoadSchema(resolveAgainst(g.root, filepath.Join("codegen", "schema_snapshots", "new.json")))
		if err != nil {
			return err
		}
		g.oldSchema, g.newSchema = old, nw
	}
	// Alias resources carry old state under a different type name; use it for the
	// old-schema (attribute passthrough) lookup.
	oldType := tfType
	if t.OldType != "" {
		oldType = t.OldType
	}
	out, err := migrate.GenerateUpgrade(g.root, tfType, t, g.oldSchema[oldType], g.newSchema[tfType])
	if err != nil {
		return err
	}
	path := filepath.Join(g.root, "internal", "service", name, name+"_upgrade_gen.go")
	if err := g.writeFile(path, out, force); err != nil {
		return err
	}

	// Emit the decode-clean golden test alongside the upgrader.
	testOut, err := migrate.GenerateUpgradeTest(g.root, tfType, t, g.oldSchema[oldType], g.newSchema[tfType])
	if err != nil {
		return err
	}
	testPath := filepath.Join(g.root, "internal", "service", name, name+"_upgrade_gen_test.go")
	return g.writeFile(testPath, testOut, force)
}

// writeFile writes data to path, refusing to overwrite an existing file unless
// force is set.
func (g *generator) writeFile(path string, data []byte, force bool) error {
	if !force {
		if _, err := g.fs.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return g.fs.WriteFile(path, data, 0o600)
}

// findProjectRoot walks up from the working directory to the module root.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

func resolveAgainst(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func sortedKeys[V any](m map[string]V) []string {
	ks := slices.Collect(maps.Keys(m))
	slices.Sort(ks)
	return ks
}
