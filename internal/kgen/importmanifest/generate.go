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

// archetypeInfo is the slice of a crud_archetypes.yaml entry Build needs: the
// archetype kind plus the two association fields (association.gtpl reads the
// same two off the same struct — see internal/kgen/crud/assoc.go).
type archetypeInfo struct {
	Kind        string
	KeyField    string
	ParentField string
}

// Build assembles the manifest from already-loaded inputs. Pure: no I/O, so the
// table-driven tests do not need a filesystem.
//
//	readPaths  kind -> read path        (generator_config.yaml)
//	archetypes kind -> archetype fields (crud_archetypes.yaml: kind, key_field, parent_field)
//	tfTypes    every managed resource the provider serves (schema snapshot)
func Build(readPaths map[string]string, archetypes map[string]archetypeInfo, tfTypes []string) *Manifest {
	out := make([]Resource, 0, len(tfTypes))

	for _, tfType := range tfTypes {
		kind := strings.TrimPrefix(tfType, "kion_")
		info := archetypes[kind]
		archetype := info.Kind
		if archetype == "" {
			archetype = "entity"
		}

		readPath := readPaths[kind]
		if readPath == "" {
			readPath = ExtraListPaths[kind]
		}

		// hasParent decides the import-id FORMAT. It must come from
		// crud_archetypes.yaml's declared parent_field, not from ParentPaths --
		// internal/kgen/crud/assoc.go derives association.gtpl's HasParent the
		// same way (arch.ParentField != "", erroring "parent_field %q not in
		// model" otherwise), and the manifest exists to mirror that template.
		hasParent := info.ParentField != ""

		shape, format, readable, reason := Classify(archetype, readPath, hasParent)

		r := Resource{
			TFType:    tfType,
			Kind:      kind,
			Archetype: archetype,
			ReadShape: shape,
			Readable:  readable,
			NameField: "name",
			ImportID:  ImportID{Format: format},
			Reason:    reason,
		}

		// The Parent block decides which ENDPOINTS the enumerator calls, which
		// is a different concern from hasParent above: it comes from
		// importmanifest.ParentPaths (authored; codegen has no equivalent).
		// Attach it whenever an entry exists, independent of read shape --
		// kion_compliance_family/level classify as ShapeGeneric (they have real
		// codegen read paths) but their flat lists 405 on real installs, so the
		// enumerator needs this fallback endpoint present regardless.
		if p, ok := ParentPaths[tfType]; ok {
			parent := p
			r.Parent = &parent
		}

		switch shape {
		case ShapeGeneric:
			r.ListPath = ListPathFrom(readPath)
		case ShapeSpecial:
			if p, ok := SpecialPaths[tfType]; ok {
				r.ListPath = p
			} else {
				r.ListPath = ListPathFrom(readPath)
			}
		case ShapeParentList, ShapeAssociation:
			if r.Parent == nil {
				if p, ok := GlobalAssociationPaths[tfType]; ok {
					r.ListPath = p
				} else {
					r.Readable = false
					r.ReadShape = ShapeNone
					r.Reason = "no entry in importmanifest.ParentPaths or GlobalAssociationPaths"
				}
			}
			// association.gtpl's ImportState only splits req.ID on "/" in its
			// {{if .HasParent}} branch; the {{else}} branch (parentless
			// associations, e.g. kion_global_permission_mapping) parses req.ID
			// as a plain id. Only claim the parent/key format when there
			// actually is a parent to split off.
			if shape == ShapeAssociation && hasParent {
				r.ImportID.KeyField = info.KeyField
			}
		}

		out = append(out, r)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].TFType < out[j].TFType })
	return &Manifest{Version: ManifestVersion, Resources: out}
}

// Generate reads the codegen inputs from root and writes OutputPath.
func Generate(fsw kfs.FS, root string) (*Manifest, error) {
	readPaths, err := loadReadPaths(fsw, filepath.Join(root, "codegen", "generator_config.yaml"))
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

	m := Build(readPaths, archetypes, tfTypes)
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
