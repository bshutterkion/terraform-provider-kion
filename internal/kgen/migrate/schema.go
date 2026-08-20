// Package migrate holds the old→new state-migration codegen: it reads the two
// providers' `terraform providers schema -json` snapshots, diffs them, and (with
// codegen/state_upgrades.yaml) generates per-resource Terraform state upgraders
// plus drives the kmigrate HCL config rewriter.
package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Attr is one attribute or nested block of a resource schema.
type Attr struct {
	Kind      string // "attr" | "block"
	TypeJSON  string // attr: the cty type as JSON (e.g. `"string"`, `["list",...]`)
	Nesting   string // block: nesting_mode (e.g. "list", "set", "single")
	Optional  bool   // attr may be set in config
	Required  bool   // attr must be set in config
	Computed  bool   // attr is set by the provider
	NestedObj bool   // attr: has a nested_type (object/collection of objects)
	// Nested is the nested_type's nesting mode and attribute names, so a
	// config-rewrite table that names fields inside a nested object (cft's
	// tag_key/tag_value) can be checked against the schema.
	NestedMode  string
	NestedAttrs []string
}

// Settable reports whether a user can write this attribute in config (so a
// dropped settable attribute is a config-migration concern, whereas a dropped
// computed-only attribute never appears in a customer's .tf).
func (a Attr) Settable() bool { return a.Optional || a.Required }

// Resource is a resource type's attribute set.
type Resource struct {
	Attrs map[string]Attr
}

// LoadSchema parses a `terraform providers schema -json` file into resources
// keyed by type (e.g. "kion_label").
func LoadSchema(path string) (map[string]Resource, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		ProviderSchemas map[string]struct {
			ResourceSchemas map[string]struct {
				Block struct {
					Attributes map[string]struct {
						Type       json.RawMessage `json:"type"`
						NestedType json.RawMessage `json:"nested_type"`
						Optional   bool            `json:"optional"`
						Required   bool            `json:"required"`
						Computed   bool            `json:"computed"`
					} `json:"attributes"`
					BlockTypes map[string]struct {
						NestingMode string `json:"nesting_mode"`
					} `json:"block_types"`
				} `json:"block"`
			} `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := map[string]Resource{}
	for _, ps := range doc.ProviderSchemas {
		for rtype, rs := range ps.ResourceSchemas {
			r := Resource{Attrs: map[string]Attr{}}
			for name, a := range rs.Block.Attributes {
				attr := Attr{
					Kind:      "attr",
					TypeJSON:  string(a.Type),
					Optional:  a.Optional,
					Required:  a.Required,
					Computed:  a.Computed,
					NestedObj: len(a.NestedType) > 0,
				}
				if attr.NestedObj {
					var nt struct {
						NestingMode string                     `json:"nesting_mode"`
						Attributes  map[string]json.RawMessage `json:"attributes"`
					}
					if err := json.Unmarshal(a.NestedType, &nt); err != nil {
						return nil, fmt.Errorf("parse %s: nested_type of %s.%s: %w", path, rtype, name, err)
					}
					attr.NestedMode = nt.NestingMode
					for n := range nt.Attributes {
						attr.NestedAttrs = append(attr.NestedAttrs, n)
					}
					sort.Strings(attr.NestedAttrs)
				}
				r.Attrs[name] = attr
			}
			for name, b := range rs.Block.BlockTypes {
				r.Attrs[name] = Attr{Kind: "block", Nesting: b.NestingMode}
			}
			out[rtype] = r
		}
		break // the snapshot contains a single provider
	}
	return out, nil
}
