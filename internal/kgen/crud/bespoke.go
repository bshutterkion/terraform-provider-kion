package crud

import (
	_ "embed"
	"fmt"
	"path/filepath"
)

// Bespoke archetypes cover resources whose CRUD control flow fits none of the
// derived archetypes (entity/association/parent_list/…): a settings singleton,
// the cloud-account convert/move state machine, and the entity-polymorphic
// custom-variable override. Their resource + data source bodies are emitted
// verbatim from per-resource templates so a wipe + regen reproduces them, while
// their `*_schema_gen.go` / `*_version_gen.go` still come from the schema and
// version stages like every other resource. service_package.go is owned by
// kgen service (it registers the hand-kept data source alongside the resource),
// so the bespoke paths do not emit it.

//go:embed singleton.gtpl
var singletonResourceTmpl string

//go:embed singleton_ds.gtpl
var singletonDataSourceTmpl string

//go:embed cvoverride.gtpl
var cvOverrideResourceTmpl string

//go:embed cvoverride_ds.gtpl
var cvOverrideDataSourceTmpl string

//go:embed cloud_account.gtpl
var cloudAccountResourceTmpl string

//go:embed cloud_account_ds_account.gtpl
var cloudAccountDSAccountTmpl string

//go:embed cloud_account_ds_aws.gtpl
var cloudAccountDSAwsTmpl string

// accounthelper is a shared support package (convert/move/read/delete/status)
// imported by the cloud_account resources. It is emitted verbatim alongside them
// so a wipe of internal/service/* reproduces it.
var (
	//go:embed accounthelper/accounthelper.gtpl
	accountHelperMainTmpl string
	//go:embed accounthelper/convert.gtpl
	accountHelperConvertTmpl string
	//go:embed accounthelper/delete.gtpl
	accountHelperDeleteTmpl string
	//go:embed accounthelper/read.gtpl
	accountHelperReadTmpl string
	//go:embed accounthelper/status.gtpl
	accountHelperStatusTmpl string
)

// Hand-kept companion files (data sources for resource-only archetypes, their
// helpers, and alias registrations) emitted verbatim next to a generated
// resource. The data-source-only gcp_regions package is handled separately.
var (
	//go:embed gcp_regions_ds.gtpl
	gcpRegionsDSTmpl string
	//go:embed ou_permission_mapping_ds.gtpl
	ouPermissionMappingDSTmpl string
	//go:embed ou_permission_mapping_helpers.gtpl
	ouPermissionMappingHelpersTmpl string
	//go:embed project_permission_mapping_ds.gtpl
	projectPermissionMappingDSTmpl string
	//go:embed project_permission_mapping_helpers.gtpl
	projectPermissionMappingHelpersTmpl string
	//go:embed funding_source_permission_mapping_ds.gtpl
	fundingSourcePermissionMappingDSTmpl string
	//go:embed funding_source_permission_mapping_helpers.gtpl
	fundingSourcePermissionMappingHelpersTmpl string
	//go:embed global_permission_mapping_ds.gtpl
	globalPermissionMappingDSTmpl string
	//go:embed global_permission_mapping_helpers.gtpl
	globalPermissionMappingHelpersTmpl string
	//go:embed project_enforcement_ds.gtpl
	projectEnforcementDSTmpl string
	//go:embed project_note_ds.gtpl
	projectNoteDSTmpl string
	//go:embed cft_alias.gtpl
	cftAliasTmpl string
	//go:embed iam_policy_alias.gtpl
	iamPolicyAliasTmpl string
	//go:embed cached_account_alias.gtpl
	cachedAccountAliasTmpl string
)

// companionsByName maps a generated resource to the hand-authored companion
// files kept beside it (its archetype does not derive them). emitCompanions
// writes these verbatim after the resource is generated, so a wipe reproduces
// them. The map is populated in an init to reference the embedded templates.
var companionsByName map[string][]bespokeFile

func init() {
	companionsByName = map[string][]bespokeFile{
		"ou_permission_mapping": {
			{ouPermissionMappingDSTmpl, "ou_permission_mapping_data_source.go"},
			{ouPermissionMappingHelpersTmpl, "data_source_helpers.go"},
		},
		"project_permission_mapping": {
			{projectPermissionMappingDSTmpl, "project_permission_mapping_data_source.go"},
			{projectPermissionMappingHelpersTmpl, "data_source_helpers.go"},
		},
		"funding_source_permission_mapping": {
			{fundingSourcePermissionMappingDSTmpl, "funding_source_permission_mapping_data_source.go"},
			{fundingSourcePermissionMappingHelpersTmpl, "data_source_helpers.go"},
		},
		"global_permission_mapping": {
			{globalPermissionMappingDSTmpl, "global_permission_mapping_data_source.go"},
			{globalPermissionMappingHelpersTmpl, "data_source_helpers.go"},
		},
		"project_enforcement": {
			{projectEnforcementDSTmpl, "project_enforcement_data_source.go"},
		},
		"project_note": {
			{projectNoteDSTmpl, "project_note_data_source.go"},
		},
		"cft": {
			{cftAliasTmpl, "aws_cloudformation_template_alias.go"},
		},
		"iam_policy": {
			{iamPolicyAliasTmpl, "aws_iam_policy_alias.go"},
		},
		"account_cache": {
			{cachedAccountAliasTmpl, "cached_account_alias.go"},
		},
	}
}

const (
	// singletonKind is a PATCH-only settings singleton (app_config): no create/
	// delete endpoint, a single record patched in place, static id, all
	// attributes Optional+Computed.
	singletonKind = "singleton"

	// cloudAccountKind is the account/aws_account convert-and-move state machine
	// (create-new-in-cache, cache<->project convert, project->project move),
	// backed by the shared accounthelper package.
	cloudAccountKind = "cloud_account"

	// cvOverrideKind is the entity-polymorphic custom_variable_override
	// (account/ou/project parent, jx.Raw value polymorphism).
	cvOverrideKind = "cv_override"

	// datasourceOnlyKind is a package with only a hand-kept data source and no
	// resource (gcp_regions). Its data source body is emitted verbatim; its
	// schema still comes from the schema stage.
	datasourceOnlyKind = "datasource_only"
)

// datasourceOnlyTemplates maps a datasource_only package to its verbatim data
// source template.
var datasourceOnlyTemplates = map[string]string{
	"gcp_regions": gcpRegionsDSTmpl,
}

// isBespokeKind reports whether a kind is emitted from verbatim per-resource
// templates rather than derived from the SDK op-set. Bespoke resources may be
// declared only in crud_archetypes.yaml (absent from generator_config
// `resources`), so the generate loop merges them in explicitly.
func isBespokeKind(kind string) bool {
	switch kind {
	case singletonKind, cloudAccountKind, cvOverrideKind, datasourceOnlyKind:
		return true
	default:
		return false
	}
}

func (g *generator) generateSingleton(dir, name string, force bool) (int, error) {
	return g.emitBespoke(dir, name, nil, []bespokeFile{
		{singletonResourceTmpl, name + ".go"},
		{singletonDataSourceTmpl, name + "_data_source.go"},
	}, force)
}

func (g *generator) generateCVOverride(dir, name string, force bool) (int, error) {
	return g.emitBespoke(dir, name, nil, []bespokeFile{
		{cvOverrideResourceTmpl, name + ".go"},
		{cvOverrideDataSourceTmpl, name + "_data_source.go"},
	}, force)
}

// cloudAccountData parameterizes the shared account/aws_account resource
// template. Recv is the lowerCamel receiver base ("account"/"awsAccount"),
// Pascal the exported base ("Account"/"AwsAccount"), ResName the human name.
type cloudAccountData struct {
	Pkg, Recv, Pascal, ResName string
	SchemaVersion              int
}

// generateCloudAccount emits the shared convert-and-move resource plus the
// resource-specific data source (account = list+filter, aws_account = by-id).
func (g *generator) generateCloudAccount(dir, name string, force bool) (int, error) {
	pascal := pascalCase(name)
	data := cloudAccountData{
		Pkg:     name,
		Recv:    lowerCamelCase(name),
		Pascal:  pascal,
		ResName: spaceBeforeCaps(pascal),
	}
	if _, ok := g.upgrades["kion_"+name]; ok {
		data.SchemaVersion = 1
	}
	dsTmpl := cloudAccountDSAwsTmpl
	if name == "account" {
		dsTmpl = cloudAccountDSAccountTmpl
	}
	if _, err := g.emitBespoke(dir, name, data, []bespokeFile{
		{cloudAccountResourceTmpl, name + ".go"},
		{dsTmpl, name + "_data_source.go"},
	}, force); err != nil {
		return 0, err
	}
	// Emit the shared accounthelper package once (when generating account).
	if name == "account" {
		if err := g.emitAccountHelper(filepath.Join(filepath.Dir(dir), "accounthelper"), force); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

// emitAccountHelper writes the shared accounthelper support package verbatim.
func (g *generator) emitAccountHelper(dir string, force bool) error {
	_, err := g.emitBespoke(dir, "accounthelper", nil, []bespokeFile{
		{accountHelperMainTmpl, "accounthelper.go"},
		{accountHelperConvertTmpl, "convert.go"},
		{accountHelperDeleteTmpl, "delete.go"},
		{accountHelperReadTmpl, "read.go"},
		{accountHelperStatusTmpl, "status.go"},
	}, force)
	return err
}

// generateDatasourceOnly emits a package that has only a hand-kept data source
// (gcp_regions) verbatim.
func (g *generator) generateDatasourceOnly(dir, name string, force bool) (int, error) {
	tmpl, ok := datasourceOnlyTemplates[name]
	if !ok {
		return 0, fmt.Errorf("%s: datasource_only archetype but no template registered", name)
	}
	return g.emitBespoke(dir, name, nil, []bespokeFile{
		{tmpl, name + "_data_source.go"},
	}, force)
}

// emitCompanions writes the hand-authored companion files kept beside a
// generated resource (data source, its helpers, alias registrations), if any
// are registered for name.
func (g *generator) emitCompanions(dir, name string, force bool) error {
	files, ok := companionsByName[name]
	if !ok {
		return nil
	}
	_, err := g.emitBespoke(dir, name, nil, files, force)
	return err
}

// bespokeFile pairs an embedded template with its output file name.
type bespokeFile struct {
	tmpl    string
	outName string
}

// emitBespoke renders each template with data and writes it into dir.
func (g *generator) emitBespoke(dir, name string, data any, files []bespokeFile, force bool) (int, error) {
	for _, f := range files {
		out, err := execGoTemplate(name+":"+f.outName, f.tmpl, data, f.outName)
		if err != nil {
			return 0, err
		}
		if err := g.writeFile(filepath.Join(dir, f.outName), out, force); err != nil {
			return 0, err
		}
	}
	return 1, nil
}
