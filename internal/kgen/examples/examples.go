// Package examples generates example .tf files from registered resource and data source schemas.
package examples

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rsschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"terraform-provider-kion/internal/provider"
)

// Generate generates example .tf files for all registered resources and data sources.
// If filterResource is non-empty, only that resource/data source is generated.
func Generate(force bool, filterResource string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pkgs := provider.ServicePackages()

	var generated int
	for _, pkg := range pkgs {
		for _, r := range pkg.Resources(ctx) {
			res := r.Factory()
			typeName := resourceTypeName(ctx, res)
			if filterResource != "" && typeName != filterResource {
				continue
			}
			if err := generateResource(projectRoot, typeName, res, force); err != nil {
				return fmt.Errorf("generating resource example for %s: %w", typeName, err)
			}
			generated++
		}
		for _, d := range pkg.DataSources(ctx) {
			ds := d.Factory()
			typeName := dataSourceTypeName(ctx, ds)
			if filterResource != "" && typeName != filterResource {
				continue
			}
			if err := generateDataSource(projectRoot, typeName, ds, force); err != nil {
				return fmt.Errorf("generating data source example for %s: %w", typeName, err)
			}
			generated++
		}
	}

	if filterResource != "" && generated == 0 {
		return fmt.Errorf("no resource or data source found matching %q", filterResource)
	}

	fmt.Printf("Generated %d example file(s)\n", generated)
	return nil
}

func generateResource(projectRoot, typeName string, r resource.Resource, force bool) error {
	dir := filepath.Join(projectRoot, "examples", "resources", typeName)
	outFile := filepath.Join(dir, "resource.tf")

	if !force {
		if _, err := fsw.Stat(outFile); err == nil {
			fmt.Printf("  skip %s (exists, use --force to overwrite)\n", outFile)
			return nil
		}
	}

	// Get the schema by calling Schema() with a dummy request/response.
	schemaReq := resource.SchemaRequest{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), schemaReq, schemaResp)

	content := resourceSchemaToTF(typeName, schemaResp.Schema)

	if err := fsw.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := fsw.WriteFile(outFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing %s: %w", outFile, err)
	}
	fmt.Printf("  wrote %s\n", outFile)
	return nil
}

func generateDataSource(projectRoot, typeName string, d datasource.DataSource, force bool) error {
	dir := filepath.Join(projectRoot, "examples", "data-sources", typeName)
	outFile := filepath.Join(dir, "data-source.tf")

	if !force {
		if _, err := fsw.Stat(outFile); err == nil {
			fmt.Printf("  skip %s (exists, use --force to overwrite)\n", outFile)
			return nil
		}
	}

	schemaReq := datasource.SchemaRequest{}
	schemaResp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), schemaReq, schemaResp)

	content := dataSourceSchemaToTF(typeName, schemaResp.Schema)

	if err := fsw.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := fsw.WriteFile(outFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing %s: %w", outFile, err)
	}
	fmt.Printf("  wrote %s\n", outFile)
	return nil
}

// resourceSchemaToTF converts a resource schema to a .tf file string.
func resourceSchemaToTF(typeName string, s rsschema.Schema) string {
	var b strings.Builder
	fmt.Fprintf(&b, "resource %q %q {\n", typeName, "example")

	required, optional := renderResourceAttrs(s.Attributes, "  ")
	blocks := renderResourceBlocks(s.Blocks, "  ")

	if len(required) > 0 {
		b.WriteString("  # Required\n")
		for _, line := range required {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if len(optional) > 0 {
		if len(required) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  # Optional\n")
		for _, line := range optional {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if len(blocks) > 0 {
		if len(required) > 0 || len(optional) > 0 {
			b.WriteString("\n")
		}
		for _, line := range blocks {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("}\n")
	return b.String()
}

// dataSourceSchemaToTF converts a data source schema to a .tf file string.
func dataSourceSchemaToTF(typeName string, s dsschema.Schema) string {
	var b strings.Builder
	fmt.Fprintf(&b, "data %q %q {\n", typeName, "example")

	required, optional := renderDataSourceAttrs(s.Attributes, "  ")
	blocks := renderDataSourceBlocks(s.Blocks, "  ")

	if len(required) > 0 {
		b.WriteString("  # Required\n")
		for _, line := range required {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if len(optional) > 0 {
		if len(required) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  # Optional\n")
		for _, line := range optional {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if len(blocks) > 0 {
		if len(required) > 0 || len(optional) > 0 {
			b.WriteString("\n")
		}
		for _, line := range blocks {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("}\n")
	return b.String()
}

// renderResourceAttrs renders resource schema attributes, returning required and optional lines.
// renderNestedResourceAttr renders a nested attribute as a multi-line HCL object
// rather than a scalar placeholder. Without this the example generator emitted
// nothing at all for nested attributes (they fell through the required/optional
// predicates), so a resource whose whole shape is a nested block — a billing
// source's aws_connection / azure_connection — got an example showing only its
// scalars and none of the part a practitioner actually needs.
//
// commented renders the whole block behind "# " for an optional attribute,
// matching how optional scalars are emitted.
func renderNestedResourceAttr(name string, nested map[string]rsschema.Attribute, indent string, commented bool) []string {
	prefix := ""
	if commented {
		prefix = "# "
	}
	lines := []string{fmt.Sprintf("%s%s%s = {", indent, prefix, name)}
	inner := indent + "  "
	req, opt := renderResourceAttrs(nested, inner)
	// Inside an already-commented block the sub-attributes are commented twice
	// over; strip the inner marker so the result reads as one commented block.
	clean := func(ls []string) []string {
		out := make([]string, 0, len(ls))
		for _, l := range ls {
			if commented {
				l = strings.Replace(l, inner+"# ", inner, 1)
				l = indent + prefix + strings.TrimPrefix(l, indent)
			}
			out = append(out, l)
		}
		return out
	}
	lines = append(lines, clean(req)...)
	lines = append(lines, clean(opt)...)
	lines = append(lines, fmt.Sprintf("%s%s}", indent, prefix))
	return lines
}

// nestedResourceAttrs returns the child attributes of a nested attribute, and
// whether attr was nested at all.
func nestedResourceAttrs(attr rsschema.Attribute) (map[string]rsschema.Attribute, bool) {
	switch a := attr.(type) {
	case rsschema.SingleNestedAttribute:
		return a.Attributes, true
	case rsschema.ListNestedAttribute:
		return a.NestedObject.Attributes, true
	case rsschema.SetNestedAttribute:
		return a.NestedObject.Attributes, true
	case rsschema.MapNestedAttribute:
		return a.NestedObject.Attributes, true
	}
	return nil, false
}

func renderResourceAttrs(attrs map[string]rsschema.Attribute, indent string) (required, optional []string) {
	names := sortedKeys(attrs)

	// Calculate padding for alignment.
	maxReqLen := 0
	maxOptLen := 0
	for _, name := range names {
		attr := attrs[name]
		if isResourceComputedOnly(attr) {
			continue
		}
		if isResourceRequired(attr) {
			if len(name) > maxReqLen {
				maxReqLen = len(name)
			}
		} else if isResourceOptional(attr) {
			if len(name) > maxOptLen {
				maxOptLen = len(name)
			}
		}
	}

	for _, name := range names {
		attr := attrs[name]
		if isResourceComputedOnly(attr) {
			continue
		}
		if nested, isNested := nestedResourceAttrs(attr); isNested {
			if isResourceRequired(attr) {
				required = append(required, renderNestedResourceAttr(name, nested, indent, false)...)
			} else if isResourceOptional(attr) {
				optional = append(optional, renderNestedResourceAttr(name, nested, indent, true)...)
			}
			continue
		}
		placeholder := resourcePlaceholder(attr)
		if isResourceRequired(attr) {
			required = append(required, fmt.Sprintf("%s%-*s = %s", indent, maxReqLen, name, placeholder))
		} else if isResourceOptional(attr) {
			optional = append(optional, fmt.Sprintf("%s# %-*s = %s", indent, maxOptLen, name, placeholder))
		}
	}
	return required, optional
}

// renderDataSourceAttrs renders data source schema attributes, returning required and optional lines.
func renderDataSourceAttrs(attrs map[string]dsschema.Attribute, indent string) (required, optional []string) {
	names := sortedKeys(attrs)

	maxReqLen := 0
	maxOptLen := 0
	for _, name := range names {
		attr := attrs[name]
		if isDataSourceComputedOnly(attr) {
			continue
		}
		if isDataSourceRequired(attr) {
			if len(name) > maxReqLen {
				maxReqLen = len(name)
			}
		} else if isDataSourceOptional(attr) {
			if len(name) > maxOptLen {
				maxOptLen = len(name)
			}
		}
	}

	for _, name := range names {
		attr := attrs[name]
		if isDataSourceComputedOnly(attr) {
			continue
		}
		placeholder := dataSourcePlaceholder(attr)
		if isDataSourceRequired(attr) {
			required = append(required, fmt.Sprintf("%s%-*s = %s", indent, maxReqLen, name, placeholder))
		} else if isDataSourceOptional(attr) {
			optional = append(optional, fmt.Sprintf("%s# %-*s = %s", indent, maxOptLen, name, placeholder))
		}
	}
	return required, optional
}

// renderResourceBlocks renders resource schema blocks as commented-out sections.
func renderResourceBlocks(blocks map[string]rsschema.Block, indent string) []string {
	names := sortedKeys(blocks)
	var lines []string

	for _, name := range names {
		block := blocks[name]
		lines = append(lines, renderOneResourceBlock(name, block, indent)...)
	}
	return lines
}

func renderOneResourceBlock(name string, block rsschema.Block, indent string) []string {
	var lines []string
	var attrs map[string]rsschema.Attribute
	var nestedBlocks map[string]rsschema.Block

	switch b := block.(type) {
	case rsschema.SingleNestedBlock:
		attrs = b.Attributes
		nestedBlocks = b.Blocks
	case rsschema.ListNestedBlock:
		attrs = b.NestedObject.Attributes
		nestedBlocks = b.NestedObject.Blocks
	case rsschema.SetNestedBlock:
		attrs = b.NestedObject.Attributes
		nestedBlocks = b.NestedObject.Blocks
	default:
		return lines
	}

	lines = append(lines, fmt.Sprintf("%s# %s {", indent, name))

	// Render attributes inside the block (all commented out).
	attrNames := sortedKeys(attrs)
	maxLen := 0
	for _, an := range attrNames {
		if isResourceComputedOnly(attrs[an]) {
			continue
		}
		if len(an) > maxLen {
			maxLen = len(an)
		}
	}
	for _, an := range attrNames {
		attr := attrs[an]
		if isResourceComputedOnly(attr) {
			continue
		}
		placeholder := resourcePlaceholder(attr)
		lines = append(lines, fmt.Sprintf("%s#   %-*s = %s", indent, maxLen, an, placeholder))
	}

	// Render nested blocks recursively.
	if len(nestedBlocks) > 0 {
		lines = append(lines, renderResourceBlocks(nestedBlocks, indent+"  ")...)
	}

	lines = append(lines, fmt.Sprintf("%s# }", indent))
	return lines
}

// renderDataSourceBlocks renders data source schema blocks as commented-out sections.
// Blocks whose attributes are all computed (read-only outputs, e.g. the common
// "list" block) are skipped so that examples only show user-configurable input.
func renderDataSourceBlocks(blocks map[string]dsschema.Block, indent string) []string {
	names := sortedKeys(blocks)
	var lines []string

	for _, name := range names {
		block := blocks[name]
		if isDataSourceBlockReadOnly(block) {
			continue
		}
		lines = append(lines, renderOneDataSourceBlock(name, block, indent)...)
	}
	return lines
}

// isDataSourceBlockReadOnly reports whether a data source block contains only
// computed attributes (and likewise read-only nested blocks), meaning it cannot
// be used as input in a data source configuration.
func isDataSourceBlockReadOnly(block dsschema.Block) bool {
	var attrs map[string]dsschema.Attribute
	var nestedBlocks map[string]dsschema.Block

	switch b := block.(type) {
	case dsschema.SingleNestedBlock:
		attrs = b.Attributes
		nestedBlocks = b.Blocks
	case dsschema.ListNestedBlock:
		attrs = b.NestedObject.Attributes
		nestedBlocks = b.NestedObject.Blocks
	case dsschema.SetNestedBlock:
		attrs = b.NestedObject.Attributes
		nestedBlocks = b.NestedObject.Blocks
	default:
		return false
	}

	for _, a := range attrs {
		if !isDataSourceComputedOnly(a) {
			return false
		}
	}
	for _, nb := range nestedBlocks {
		if !isDataSourceBlockReadOnly(nb) {
			return false
		}
	}
	return true
}

func renderOneDataSourceBlock(name string, block dsschema.Block, indent string) []string {
	var lines []string
	var attrs map[string]dsschema.Attribute
	var nestedBlocks map[string]dsschema.Block

	switch b := block.(type) {
	case dsschema.SingleNestedBlock:
		attrs = b.Attributes
		nestedBlocks = b.Blocks
	case dsschema.ListNestedBlock:
		attrs = b.NestedObject.Attributes
		nestedBlocks = b.NestedObject.Blocks
	case dsschema.SetNestedBlock:
		attrs = b.NestedObject.Attributes
		nestedBlocks = b.NestedObject.Blocks
	default:
		return lines
	}

	lines = append(lines, fmt.Sprintf("%s# %s {", indent, name))

	attrNames := sortedKeys(attrs)
	maxLen := 0
	for _, an := range attrNames {
		if isDataSourceComputedOnly(attrs[an]) {
			continue
		}
		if len(an) > maxLen {
			maxLen = len(an)
		}
	}
	for _, an := range attrNames {
		attr := attrs[an]
		if isDataSourceComputedOnly(attr) {
			continue
		}
		placeholder := dataSourcePlaceholder(attr)
		lines = append(lines, fmt.Sprintf("%s#   %-*s = %s", indent, maxLen, an, placeholder))
	}

	if len(nestedBlocks) > 0 {
		lines = append(lines, renderDataSourceBlocks(nestedBlocks, indent+"  ")...)
	}

	lines = append(lines, fmt.Sprintf("%s# }", indent))
	return lines
}

// --- Resource attribute helpers ---

func isResourceRequired(attr rsschema.Attribute) bool {
	switch a := attr.(type) {
	case rsschema.StringAttribute:
		return a.Required
	case rsschema.Int64Attribute:
		return a.Required
	case rsschema.BoolAttribute:
		return a.Required
	case rsschema.Float64Attribute:
		return a.Required
	case rsschema.MapAttribute:
		return a.Required
	case rsschema.ListAttribute:
		return a.Required
	case rsschema.SetAttribute:
		return a.Required
	case rsschema.SingleNestedAttribute:
		return a.Required
	case rsschema.ListNestedAttribute:
		return a.Required
	case rsschema.SetNestedAttribute:
		return a.Required
	case rsschema.MapNestedAttribute:
		return a.Required
	default:
		return false
	}
}

func isResourceOptional(attr rsschema.Attribute) bool {
	switch a := attr.(type) {
	case rsschema.StringAttribute:
		return a.Optional
	case rsschema.Int64Attribute:
		return a.Optional
	case rsschema.BoolAttribute:
		return a.Optional
	case rsschema.Float64Attribute:
		return a.Optional
	case rsschema.MapAttribute:
		return a.Optional
	case rsschema.ListAttribute:
		return a.Optional
	case rsschema.SetAttribute:
		return a.Optional
	case rsschema.SingleNestedAttribute:
		return a.Optional
	case rsschema.ListNestedAttribute:
		return a.Optional
	case rsschema.SetNestedAttribute:
		return a.Optional
	case rsschema.MapNestedAttribute:
		return a.Optional
	default:
		return false
	}
}

func isResourceComputedOnly(attr rsschema.Attribute) bool {
	switch a := attr.(type) {
	case rsschema.StringAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.Int64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.BoolAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.Float64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.MapAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.ListAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.SetAttribute:
		return a.Computed && !a.Required && !a.Optional
	case rsschema.SingleNestedAttribute:
		return a.Computed && !a.Optional && !a.Required
	case rsschema.ListNestedAttribute:
		return a.Computed && !a.Optional && !a.Required
	case rsschema.SetNestedAttribute:
		return a.Computed && !a.Optional && !a.Required
	case rsschema.MapNestedAttribute:
		return a.Computed && !a.Optional && !a.Required
	default:
		return false
	}
}

func resourcePlaceholder(attr rsschema.Attribute) string {
	switch attr.(type) {
	case rsschema.StringAttribute:
		return `"example"`
	case rsschema.Int64Attribute:
		return "1"
	case rsschema.BoolAttribute:
		return "false"
	case rsschema.Float64Attribute:
		return "0.0"
	case rsschema.MapAttribute:
		return "{}"
	case rsschema.ListAttribute:
		return "[]"
	case rsschema.SetAttribute:
		return "[]"
	default:
		return `"example"`
	}
}

// --- Data source attribute helpers ---

func isDataSourceRequired(attr dsschema.Attribute) bool {
	switch a := attr.(type) {
	case dsschema.StringAttribute:
		return a.Required
	case dsschema.Int64Attribute:
		return a.Required
	case dsschema.BoolAttribute:
		return a.Required
	case dsschema.Float64Attribute:
		return a.Required
	case dsschema.MapAttribute:
		return a.Required
	case dsschema.ListAttribute:
		return a.Required
	case dsschema.SetAttribute:
		return a.Required
	default:
		return false
	}
}

func isDataSourceOptional(attr dsschema.Attribute) bool {
	switch a := attr.(type) {
	case dsschema.StringAttribute:
		return a.Optional
	case dsschema.Int64Attribute:
		return a.Optional
	case dsschema.BoolAttribute:
		return a.Optional
	case dsschema.Float64Attribute:
		return a.Optional
	case dsschema.MapAttribute:
		return a.Optional
	case dsschema.ListAttribute:
		return a.Optional
	case dsschema.SetAttribute:
		return a.Optional
	default:
		return false
	}
}

func isDataSourceComputedOnly(attr dsschema.Attribute) bool {
	switch a := attr.(type) {
	case dsschema.StringAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.Int64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.BoolAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.Float64Attribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.MapAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.ListAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.SetAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.ObjectAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.SingleNestedAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.ListNestedAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.SetNestedAttribute:
		return a.Computed && !a.Required && !a.Optional
	case dsschema.MapNestedAttribute:
		return a.Computed && !a.Required && !a.Optional
	default:
		return false
	}
}

func dataSourcePlaceholder(attr dsschema.Attribute) string {
	switch attr.(type) {
	case dsschema.StringAttribute:
		return `"example"`
	case dsschema.Int64Attribute:
		return "1"
	case dsschema.BoolAttribute:
		return "false"
	case dsschema.Float64Attribute:
		return "0.0"
	case dsschema.MapAttribute:
		return "{}"
	case dsschema.ListAttribute:
		return "[]"
	case dsschema.SetAttribute:
		return "[]"
	default:
		return `"example"`
	}
}

// --- Utilities ---

// sortedKeys returns the sorted keys of a map with any value type.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

const providerTypeName = "kion"

// resourceTypeName calls Metadata() on a resource to get its full type name.
func resourceTypeName(ctx context.Context, r resource.Resource) string {
	req := resource.MetadataRequest{ProviderTypeName: providerTypeName}
	resp := &resource.MetadataResponse{}
	r.Metadata(ctx, req, resp)
	return resp.TypeName
}

// dataSourceTypeName calls Metadata() on a data source to get its full type name.
func dataSourceTypeName(ctx context.Context, d datasource.DataSource) string {
	req := datasource.MetadataRequest{ProviderTypeName: providerTypeName}
	resp := &datasource.MetadataResponse{}
	d.Metadata(ctx, req, resp)
	return resp.TypeName
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	for {
		if _, err := fsw.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}
