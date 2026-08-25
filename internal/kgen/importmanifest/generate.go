package importmanifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"terraform-provider-kion/internal/kgen/kfs"
)

// OutputPath is where Generate writes, relative to the project root.
const OutputPath = "codegen/import_manifest.json"

// kindAliases covers the two kinds the provider serves under a tf_type that
// codegen does not use for its resources/data_sources keys: the schema
// snapshot strips kion_aws_cloudformation_template / kion_aws_iam_policy down
// to "aws_cloudformation_template" / "aws_iam_policy", but
// generator_config.yaml keys their entries "cft" / "iam_policy". Resolve
// through this before any generator_config.yaml lookup.
var kindAliases = map[string]string{
	"aws_cloudformation_template": "cft",
	"aws_iam_policy":              "iam_policy",
}

// archetypeInfo is the slice of a crud_archetypes.yaml entry Build needs: the
// archetype kind plus the two association fields (association.gtpl reads the
// same two off the same struct — see internal/kgen/crud/assoc.go).
type archetypeInfo struct {
	Kind        string
	KeyField    string
	ParentField string
}

// parentOverrides corrects three resources, verified against a real install,
// where placeholder-based derivation picks a parent list that isn't actually
// listable:
//
//	/v4/idms/open-id                     -> 405 (not a listable collection)
//	/v4/idms/open-id/28                  -> 200 (single OpenID-type IDMS)
//	/v4/idms/open-id/28/access-rule       -> 200
//	/v4/idms/open-id/28/group-association -> 200
//	/v3/idms                             -> 200, and it enumerates them
//
// OpenID records are keyed by the IDMS id, not their own -- so the child
// paths derivation would produce are correct (a single-object GET for
// kion_idms_open_id itself, which the reader already handles; the
// create-shaped, not read-shaped, collection endpoints for its two
// sub-resources), only the parent LIST is wrong. This table overrides just
// Kind/ListPath/ParentIDField; ChildPath is the live-verified value.
var parentOverrides = map[string]Parent{
	"kion_idms_open_id": {
		Kind: "idms", ListPath: "/v3/idms",
		ChildPath: "/v4/idms/open-id/{parent_id}", ParentIDField: "idms_id",
	},
	"kion_idms_open_id_access_rule": {
		Kind: "idms", ListPath: "/v3/idms",
		ChildPath: "/v4/idms/open-id/{parent_id}/access-rule", ParentIDField: "idms_id",
	},
	"kion_idms_open_id_group_association": {
		Kind: "idms", ListPath: "/v3/idms",
		ChildPath: "/v4/idms/open-id/{parent_id}/group-association", ParentIDField: "idms_id",
	},
}

// multiParentOverrides corrects resources enumerable under more than one
// parent collection -- Build's per-resource derivation models exactly one
// Parent, but /v3/budget 405s (there is no flat list) and budgets hang off
// two parents, both verified live: /v3/ou/{id}/budget and
// /v3/project/{id}/budget.
var multiParentOverrides = map[string][]Parent{
	"kion_budget": {
		{Kind: "ou", ListPath: "/v3/ou", ChildPath: "/v3/ou/{parent_id}/budget", ParentIDField: "ou_id"},
		{Kind: "project", ListPath: "/v3/project", ChildPath: "/v3/project/{parent_id}/budget", ParentIDField: "project_id"},
	},
}

// Build assembles the manifest from already-loaded inputs. Pure: no I/O, so the
// table-driven tests do not need a filesystem.
//
//	readPaths       kind -> by-id read path (generator_config.yaml resources:)
//	dataSourcePaths kind -> list read path  (generator_config.yaml data_sources:)
//	archetypes      kind -> archetype fields (crud_archetypes.yaml: kind, key_field, parent_field)
//	tfTypes         every managed resource the provider serves (schema snapshot)
//
// For each resource the CHOSEN PATH is the data source's read path when one
// exists (it is the real collection endpoint) and the resource's by-id read
// path otherwise -- generator_config.yaml records only a by-id read for every
// resource, so that path is right for a flat list only when nothing follows
// its "{id}"; a data source, when present, is authoritative.
//
// The chosen path's placeholder count then drives shape/list-path/parent,
// independent of archetype except where crud_archetypes.yaml fixes the shape
// outright (no_read, datasource_only, association, parent_list,
// compound_key_parent_read, singleton, raw_http, cv_override -- Classify's
// job, unchanged):
//
//   - 0 placeholders, or exactly 1 that is the path's final segment (a plain
//     by-id GET, e.g. "/v3/app-role/{id}"): flat. ListPath is the path with
//     that trailing placeholder stripped.
//   - exactly 1 placeholder NOT at the end (e.g. "/v3/ou/{id}/ou-note"):
//     parent-scoped. The text before the placeholder is the parent's list
//     path; its segments after the /vN version prefix, joined with "_" (with
//     any "-" also folded to "_"), plus "_id" name the parent id field.
//   - 2+ placeholders (kion_custom_variable_override's
//     /v3/account/{account_id}/custom-variable/{custom_variable_id}): not
//     enumerable -- neither id can be discovered without the other, so the
//     resource is unreadable regardless of what its archetype would otherwise
//     allow.
func Build(readPaths, dataSourcePaths map[string]string, archetypes map[string]archetypeInfo, tfTypes []string) *Manifest {
	out := make([]Resource, 0, len(tfTypes))

	for _, tfType := range tfTypes {
		kind := strings.TrimPrefix(tfType, "kion_")
		lookupKind := kind
		if alias, ok := kindAliases[kind]; ok {
			lookupKind = alias
		}

		// An alias reads the same endpoint as its canonical type. Record the
		// relationship and stop: enumerating both would double-import.
		if canonical, ok := kindAliases[kind]; ok {
			out = append(out, Resource{
				TFType:   tfType,
				Kind:     kind,
				AliasOf:  "kion_" + canonical,
				Readable: false,
				Reason:   "second tf_type for kion_" + canonical + "; importing both would put two resources in charge of one record",
			})
			continue
		}

		info := archetypes[lookupKind]
		archetype := info.Kind
		if archetype == "" {
			archetype = "entity"
		}

		// hasParent decides the import-id FORMAT. It must come from
		// crud_archetypes.yaml's declared parent_field, not from whether a
		// Parent block gets attached below -- internal/kgen/crud/assoc.go
		// derives association.gtpl's HasParent the same way (arch.ParentField
		// != "", erroring "parent_field %q not in model" otherwise), and the
		// manifest exists to mirror that template.
		hasParent := info.ParentField != ""

		chosenPath := dataSourcePaths[lookupKind]
		if chosenPath == "" {
			chosenPath = readPaths[lookupKind]
		}

		r := Resource{
			TFType:    tfType,
			Kind:      kind,
			Archetype: archetype,
			NameField: "name",
		}

		placeholders := strings.Count(chosenPath, "{")

		switch {
		case archetype == "no_read" || archetype == "datasource_only":
			// Fixed regardless of path: neither has a readable collection.
			r.ReadShape, r.ImportID.Format, r.Readable, r.Reason = Classify(archetype, chosenPath, hasParent)

		case placeholders >= 2:
			// Two ids in one path (custom_variable_override's
			// .../custom-variable/{custom_variable_id} under
			// account/{account_id}) can't be enumerated from either id alone.
			// This overrides even an archetype Classify would otherwise fix
			// readable (cv_override).
			r.ReadShape = ShapeNone
			r.ImportID.Format = FormatID
			r.Readable = false
			r.Reason = fmt.Sprintf("%s takes more than one id to reach -- not enumerable", chosenPath)

		case placeholders == 1 && !isTrailingPlaceholder(chosenPath):
			parent := parentFromPath(chosenPath)
			r.Parent = &parent
			r.Readable = true
			r.ImportID.Format = FormatID
			if archetype == "association" {
				r.ReadShape = ShapeAssociation
				if hasParent {
					r.ImportID.Format = FormatParentSlashKey
				}
			} else {
				r.ReadShape = ShapeParentList
			}

		default:
			// 0 placeholders, or a single trailing one (a plain by-id GET
			// that collapses to a flat list the same way ListPathFrom always
			// has). Classify's archetype-fixed branches (association,
			// parent_list, compound_key_parent_read, singleton, raw_http,
			// cv_override) and its default entity/blended/cloud_account
			// bucket both apply unchanged.
			r.ReadShape, r.ImportID.Format, r.Readable, r.Reason = Classify(archetype, chosenPath, hasParent)
			r.ListPath = ListPathFrom(chosenPath)
		}

		// association.gtpl's ImportState only splits req.ID on "/" in its
		// {{if .HasParent}} branch; the {{else}} branch (parentless
		// associations, e.g. kion_global_permission_mapping) parses req.ID as
		// a plain integer and assigns it directly to the KEY field (not a
		// synthesized id) -- so for a parentless association the import id
		// IS the key field's value, and the manifest must still carry
		// KeyField so the enumerator knows which raw field to read it from.
		// Format stays FormatID either way; only whether KeyField is set
		// depends on the association having a key field at all.
		if r.ReadShape == ShapeAssociation {
			r.ImportID.KeyField = info.KeyField
		}

		// Applied after derivation: these two tables replace a wrongly
		// derived parent (or a flat classification that never had one) with
		// the live-verified shape. See their doc comments for the evidence.
		if ov, ok := parentOverrides[tfType]; ok {
			p := ov
			r.Parent = &p
			r.ListPath = ""
			r.ReadShape = ShapeParentList
			r.Readable = true
			r.Reason = ""
		}

		if parents, ok := multiParentOverrides[tfType]; ok {
			ps := make([]Parent, len(parents))
			copy(ps, parents)
			r.Parents = ps
			r.Parent = &ps[0]
			r.ListPath = ""
			r.ReadShape = ShapeParentList
			r.Readable = true
			r.Reason = ""
		}

		out = append(out, r)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].TFType < out[j].TFType })
	return &Manifest{Version: ManifestVersion, Resources: out}
}

// isTrailingPlaceholder reports whether path's single "{...}" placeholder is
// its final path segment (a plain by-id GET, e.g. "/v3/app-role/{id}") rather
// than followed by a nested collection segment (e.g. "/v3/ou/{id}/ou-note").
func isTrailingPlaceholder(path string) bool {
	open := strings.Index(path, "{")
	if open < 0 {
		return false
	}
	closeRel := strings.Index(path[open:], "}")
	if closeRel < 0 {
		return false
	}
	return open+closeRel+1 == len(path)
}

// parentFromPath builds a Parent block from a chosen path containing exactly
// one non-trailing "{...}" placeholder, e.g. "/v3/ou/{id}/ou-note" or
// "/v4/compliance/program/{id}/control".
func parentFromPath(path string) Parent {
	open := strings.Index(path, "{")
	closeRel := strings.Index(path[open:], "}")
	closeIdx := open + closeRel + 1

	listPath := strings.TrimSuffix(path[:open], "/")
	childPath := path[:open] + "{parent_id}" + path[closeIdx:]

	// Parent id field: the list path's segments after the /vN (or /beta)
	// version prefix, joined with "_" and any "-" folded to "_", plus "_id".
	// "/v3/ou" -> "ou_id"; "/v4/compliance/program" -> "compliance_program_id";
	// "/v3/funding-source" -> "funding_source_id".
	segments := strings.Split(strings.TrimPrefix(listPath, "/"), "/")
	idField := strings.ReplaceAll(strings.Join(segments[1:], "_"), "-", "_") + "_id"

	return Parent{
		Kind:          strings.TrimSuffix(idField, "_id"),
		ListPath:      listPath,
		ChildPath:     childPath,
		ParentIDField: idField,
	}
}

// Generate reads the codegen inputs from root and writes OutputPath.
func Generate(fsw kfs.FS, root string) (*Manifest, error) {
	readPaths, err := loadReadPaths(fsw, filepath.Join(root, "codegen", "generator_config.yaml"))
	if err != nil {
		return nil, err
	}
	dataSourcePaths, err := loadDataSourceReadPaths(fsw, filepath.Join(root, "codegen", "generator_config.yaml"))
	if err != nil {
		return nil, err
	}
	archetypes, err := loadArchetypeKinds(fsw, filepath.Join(root, "codegen", "crud_archetypes.yaml"))
	if err != nil {
		return nil, err
	}
	tfTypes, err := loadTFTypes(fsw, filepath.Join(root, "codegen", "schema_snapshots", "new.json"))
	if err != nil {
		return nil, err
	}

	m := Build(readPaths, dataSourcePaths, archetypes, tfTypes)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if err := fsw.WriteFile(filepath.Join(root, OutputPath), data, 0o644); err != nil {
		return nil, err
	}
	return m, nil
}

func loadReadPaths(fsw kfs.FS, path string) (map[string]string, error) {
	raw, err := fsw.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		Resources map[string]struct {
			Read struct {
				Path string `yaml:"path"`
			} `yaml:"read"`
		} `yaml:"resources"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make(map[string]string, len(doc.Resources))
	for kind, entry := range doc.Resources {
		out[kind] = entry.Read.Path
	}
	return out, nil
}

// loadDataSourceReadPaths reads generator_config.yaml's data_sources: section,
// mirroring loadReadPaths -- the same shape one level down. Unlike a
// resource's by-id read, a data source's read path is the real list/collection
// endpoint, so it is preferred over the resource fallback whenever present.
func loadDataSourceReadPaths(fsw kfs.FS, path string) (map[string]string, error) {
	raw, err := fsw.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		DataSources map[string]struct {
			Read struct {
				Path string `yaml:"path"`
			} `yaml:"read"`
		} `yaml:"data_sources"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make(map[string]string, len(doc.DataSources))
	for kind, entry := range doc.DataSources {
		out[kind] = entry.Read.Path
	}
	return out, nil
}

func loadArchetypeKinds(fsw kfs.FS, path string) (map[string]archetypeInfo, error) {
	raw, err := fsw.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc map[string]struct {
		Kind        string `yaml:"kind"`
		KeyField    string `yaml:"key_field"`
		ParentField string `yaml:"parent_field"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make(map[string]archetypeInfo, len(doc))
	for kind, entry := range doc {
		out[kind] = archetypeInfo{
			Kind:        entry.Kind,
			KeyField:    entry.KeyField,
			ParentField: entry.ParentField,
		}
	}
	return out, nil
}

func loadTFTypes(fsw kfs.FS, path string) ([]string, error) {
	raw, err := fsw.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		ProviderSchemas map[string]struct {
			ResourceSchemas map[string]json.RawMessage `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var out []string
	for _, provider := range doc.ProviderSchemas {
		for tfType := range provider.ResourceSchemas {
			out = append(out, tfType)
		}
	}
	sort.Strings(out)
	return out, nil
}
