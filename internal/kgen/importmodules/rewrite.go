package importmodules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// Warning is emitted when an attribute or nested block present in the input
// has no corresponding module variable: a computed-only attribute the module
// never exposed as an input (last_updated is the common case), or an
// attribute the manifest simply has no entry for. Rewrite drops it rather
// than guess at a mapping; a silently dropped attribute is how a rewrite
// would quietly change what gets managed, so the caller must be told.
type Warning struct {
	Resource string // Terraform resource type, e.g. "kion_ou"
	Label    string // the resource block's second label, e.g. "example"
	Attr     string // the dropped attribute or nested block type
}

func (w Warning) String() string {
	return fmt.Sprintf("%s.%s: dropped %q -- no module variable for this attribute", w.Resource, w.Label, w.Attr)
}

// Rewrite turns every `resource "kion_*" "<label>"` block in src whose type
// has an entry in manifest into a `module "<label>" { source =
// "<modulesDir>/<path>" ... }` call wired to that module's variables. A
// resource block whose type is not in the manifest is left completely
// untouched -- byte-for-byte, including its position relative to surrounding
// content -- as are any non-resource blocks (provider, terraform, locals,
// ...).
//
// Within a converted block, each plain attribute and nested schema block is
// mapped through the manifest's attr -> var names (module.Manifest's Block
// flag says which: a nested block becomes an object-literal expression for
// its variable, matching how the module's own main.tf unwraps a single-nested
// block variable via `dynamic { for_each = var.x == null ? [] : [var.x];
// content { ... } }`). An attribute or block with no module variable is
// dropped and reported as a Warning instead of carried over.
//
// The result is run through hclwrite.Format, matching `terraform fmt`, and is
// idempotent in the sense that matters here: rewriting the same input twice
// produces byte-identical output (already-rewritten `module` blocks are not
// `resource` blocks, so a second pass leaves them alone too).
func Rewrite(src []byte, manifest *Manifest, modulesDir string) ([]byte, []Warning, error) {
	f, diags := hclwrite.ParseConfig(src, "generated.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("parse: %s", diags.Error())
	}

	byType := manifest.ByType()
	var warnings []Warning

	for _, block := range f.Body().Blocks() {
		if block.Type() != "resource" || len(block.Labels()) < 2 {
			continue
		}
		rtype, rname := block.Labels()[0], block.Labels()[1]
		mod, ok := byType[rtype]
		if !ok {
			continue
		}
		byAttr := mod.byAttr()
		body := block.Body()

		type entry struct {
			varName string
			toks    hclwrite.Tokens
		}
		var entries []entry

		attrNames := make([]string, 0, len(body.Attributes()))
		for name := range body.Attributes() {
			attrNames = append(attrNames, name)
		}
		sort.Strings(attrNames)
		for _, name := range attrNames {
			v, known := byAttr[name]
			if !known || v.Block {
				warnings = append(warnings, Warning{Resource: rtype, Label: rname, Attr: name})
				continue
			}
			entries = append(entries, entry{varName: v.Var, toks: body.GetAttribute(name).Expr().BuildTokens(nil)})
		}

		for _, nb := range body.Blocks() {
			v, known := byAttr[nb.Type()]
			if !known || !v.Block {
				warnings = append(warnings, Warning{Resource: rtype, Label: rname, Attr: nb.Type()})
				continue
			}
			entries = append(entries, entry{varName: v.Var, toks: blockToObject(nb)})
		}

		// Sorted so a given resource's variables land in a stable order
		// regardless of the input's attribute order or hclwrite's map
		// iteration -- required for the determinism Rewrite promises.
		sort.Slice(entries, func(i, j int) bool { return entries[i].varName < entries[j].varName })

		srcToks, err := stringTokens(joinSource(modulesDir, mod.Path))
		if err != nil {
			return nil, nil, fmt.Errorf("%s.%s: building source: %w", rtype, rname, err)
		}

		block.SetType("module")
		block.SetLabels([]string{rname})
		// Removed one at a time rather than with body.Clear(): Clear drops the
		// newline that follows the body's opening brace, after which every
		// SetAttributeRaw below renders onto the `{` line -- and any attribute
		// whose tokens came from this same body is dropped outright.
		for _, name := range attrNames {
			body.RemoveAttribute(name)
		}
		for _, nb := range body.Blocks() {
			body.RemoveBlock(nb)
		}
		body.SetAttributeRaw("source", srcToks)
		for _, e := range entries {
			body.SetAttributeRaw(e.varName, e.toks)
		}
	}

	return hclwrite.Format(f.Bytes()), warnings, nil
}

// CountKnownBlocks reports how many top-level `resource "kion_*" "<label>"`
// blocks in src have a manifest entry -- and so would be turned into a
// module call by Rewrite -- versus not. It does no rewriting; `kgen
// import-modules` uses it to print a one-line rewritten/untouched summary
// alongside Rewrite's warnings.
func CountKnownBlocks(src []byte, manifest *Manifest) (rewritten, untouched int, err error) {
	f, diags := hclwrite.ParseConfig(src, "generated.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return 0, 0, fmt.Errorf("parse: %s", diags.Error())
	}
	byType := manifest.ByType()
	for _, block := range f.Body().Blocks() {
		if block.Type() != "resource" || len(block.Labels()) < 2 {
			continue
		}
		if _, ok := byType[block.Labels()[0]]; ok {
			rewritten++
		} else {
			untouched++
		}
	}
	return rewritten, untouched, nil
}

// blockToObject renders a nested block's own attributes as an HCL
// object-constructor token sequence: { name = expr\n ... }. This is the model
// migrate.blockBodyToObject uses for an unrelated rewrite (existing
// state-upgrade configs); reimplemented independently here so this package
// has no dependency on that one -- the two rewrites target different inputs
// and evolving one must never risk silently changing the other.
func blockToObject(b *hclwrite.Block) hclwrite.Tokens {
	toks := hclwrite.Tokens{
		{Type: hclsyntax.TokenOBrace, Bytes: []byte("{")},
		{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
	}
	body := b.Body()
	names := make([]string, 0, len(body.Attributes()))
	for name := range body.Attributes() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		attr := body.GetAttribute(name)
		toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(name)})
		toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenEqual, Bytes: []byte(" = ")})
		toks = append(toks, attr.Expr().BuildTokens(nil)...)
		toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
	}
	toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenCBrace, Bytes: []byte("}")})
	return toks
}

// joinSource builds a module's source address as the literal
// "<modulesDir>/<path>" the module block spec calls for -- string
// concatenation, not a cleaned filepath join. path.Join (or filepath.Join)
// would Clean away a leading "./", and Terraform's source-address parser
// treats that prefix as load-bearing: "./modules/x" is a local path, but
// "modules/x" (no "./") is parsed as a registry or go-getter address instead.
// Silently mangling that would turn every rewritten module call into one
// Terraform cannot resolve.
func joinSource(modulesDir, path string) string {
	if modulesDir == "" {
		return path
	}
	return strings.TrimRight(modulesDir, "/") + "/" + path
}

// stringTokens renders s as an HCL quoted-string expression's tokens,
// round-tripped through the parser so the escaping Terraform itself uses is
// what ends up on disk.
func stringTokens(s string) (hclwrite.Tokens, error) {
	return parseExpr(fmt.Sprintf("%q", s))
}

// parseExpr parses generated expression source back into tokens, so every
// construction in this file is validated by the same parser Terraform uses
// before it can reach the rewritten output.
func parseExpr(src string) (hclwrite.Tokens, error) {
	f, diags := hclwrite.ParseConfig([]byte("x = "+src+"\n"), "importmodules-generated.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("generated %q does not parse: %s", src, diags.Error())
	}
	attr := f.Body().GetAttribute("x")
	if attr == nil {
		return nil, fmt.Errorf("generated %q does not parse as an expression", src)
	}
	return attr.Expr().BuildTokens(nil), nil
}
