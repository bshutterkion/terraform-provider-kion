// Package rewriteref turns the literal foreign keys in a generated Terraform
// configuration into references between resource blocks.
//
// `terraform plan -generate-config-out` writes every attribute as a literal,
// because Terraform cannot know that `ou_id = 11` points at anything:
//
//	resource "kion_project" "kion_project_9" {
//	  ou_id = 11
//	}
//
// That is correct for the install it was imported from, and it is the whole
// problem everywhere else. The configuration does not express its own
// dependency graph, so destroy-and-reapply does not order correctly and
// recreating an OU does not cascade; and applied against a DIFFERENT install
// the literal either dangles or -- worse -- silently binds to whatever record
// happens to hold id 11 there. Rewritten to a reference:
//
//	resource "kion_project" "kion_project_9" {
//	  ou_id = kion_ou.kion_ou_11.id
//	}
//
// Terraform's graph then does the remapping itself, and gets ordering and
// parallelism for free. This is why cloning an install needs no id map on the
// Terraform path: references are the id map.
//
// The id -> label index cannot come from generated.tf, which contains no ids
// (they are Computed, so `-generate-config-out` omits them). It comes from the
// import blocks, where `to = kion_ou.kion_ou_11` and `id = "11"` sit together.
// Both files are therefore required.
package rewriteref

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"terraform-provider-kion/internal/kgen/references"
)

// Index maps a resource id to the configuration label that manages it,
// per tf_type: index["kion_ou"]["11"] == "kion_ou_11".
type Index map[string]map[string]string

// Unresolved is one foreign key that could not be rewritten. These are the
// boundary of the configuration: the referent is not managed here, so the
// literal is left in place and reported rather than silently kept.
type Unresolved struct {
	Resource string // block label, e.g. "kion_project.kion_project_9"
	Attr     string // "owner_user_ids"
	Target   string // "kion_user"
	Value    string // the literal that could not be resolved
}

// Result reports what a rewrite did.
type Result struct {
	Rewritten  int          // literals replaced with references
	Unresolved []Unresolved // literals left alone, with why
	Blocks     int          // resource blocks visited
}

// TargetCounts summarizes Unresolved by target type, which is the actionable
// view: "47 references to kion_user could not be resolved" tells an operator
// they need to import users, or supply them another way.
func (r Result) TargetCounts() map[string]int {
	out := map[string]int{}
	for _, u := range r.Unresolved {
		out[u.Target]++
	}
	return out
}

// BuildIndex reads import blocks and returns the id -> label index.
//
// An import id may be compound ("11/4" for a parent-scoped resource, where 4 is
// the record's own id). Foreign keys hold the plain record id, so the trailing
// segment is what gets indexed; the full id is indexed too, harmlessly, so a
// compound value would also resolve.
func BuildIndex(importsHCL []byte, filename string) (Index, error) {
	p := hclparse.NewParser()
	f, diags := p.ParseHCL(importsHCL, filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing %s: %s", filename, diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("parsing %s: unexpected body type", filename)
	}

	idx := Index{}
	for _, blk := range body.Blocks {
		if blk.Type != "import" {
			continue
		}
		var to, id string
		for name, attr := range blk.Body.Attributes {
			switch name {
			case "to":
				// `to` is a traversal (kion_ou.kion_ou_11), not a string.
				if t, d := hcl.AbsTraversalForExpr(attr.Expr); !d.HasErrors() {
					parts := make([]string, 0, len(t))
					for _, step := range t {
						switch s := step.(type) {
						case hcl.TraverseRoot:
							parts = append(parts, s.Name)
						case hcl.TraverseAttr:
							parts = append(parts, s.Name)
						}
					}
					to = strings.Join(parts, ".")
				}
			case "id":
				if v, d := attr.Expr.Value(nil); !d.HasErrors() && v.Type() == cty.String {
					id = v.AsString()
				}
			}
		}
		tfType, label, ok := strings.Cut(to, ".")
		if !ok || id == "" {
			continue
		}
		if idx[tfType] == nil {
			idx[tfType] = map[string]string{}
		}
		idx[tfType][id] = label
		if _, rec, compound := strings.Cut(id, "/"); compound {
			idx[tfType][rec] = label
		}
	}
	return idx, nil
}

// Rewrite replaces resolvable literal foreign keys in generatedHCL with
// references, returning the new file content and a report.
func Rewrite(generatedHCL []byte, filename string, idx Index, refs *references.Map) ([]byte, Result, error) {
	f, diags := hclwrite.ParseConfig(generatedHCL, filename, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, Result{}, fmt.Errorf("parsing %s: %s", filename, diags.Error())
	}

	var res Result
	for _, blk := range f.Body().Blocks() {
		if blk.Type() != "resource" || len(blk.Labels()) != 2 {
			continue
		}
		tfType, label := blk.Labels()[0], blk.Labels()[1]
		res.Blocks++

		for attrName, attr := range blk.Body().Attributes() {
			target, isRef := refs.Target(tfType, attrName)
			if !isRef {
				continue
			}
			tokens := attr.Expr().BuildTokens(nil)
			newTokens, n, unresolved := rewriteExpr(tokens, target, idx)
			for _, v := range unresolved {
				res.Unresolved = append(res.Unresolved, Unresolved{
					Resource: tfType + "." + label,
					Attr:     attrName,
					Target:   target,
					Value:    v,
				})
			}
			if n == 0 {
				continue
			}
			// A self-reference would be a cycle Terraform refuses to plan. It
			// can arise from a resource whose FK points at its own record.
			if strings.Contains(string(newTokens.Bytes()), tfType+"."+label+".id") {
				continue
			}
			blk.Body().SetAttributeRaw(attrName, newTokens)
			res.Rewritten += n
		}
	}
	sort.Slice(res.Unresolved, func(i, j int) bool {
		a, b := res.Unresolved[i], res.Unresolved[j]
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.Resource != b.Resource {
			return a.Resource < b.Resource
		}
		return a.Value < b.Value
	})
	return f.Bytes(), res, nil
}

// rewriteExpr replaces each resolvable integer literal in an expression's
// tokens with a `<target>.<label>.id` traversal, preserving everything else
// (brackets, commas, whitespace) exactly as written.
//
// It works on the token stream rather than re-encoding a parsed value because
// an expression may already hold references, or a value this package should not
// touch; anything that is not a bare integer is left alone.
func rewriteExpr(tokens hclwrite.Tokens, target string, idx Index) (hclwrite.Tokens, int, []string) {
	byID := idx[target]
	var out hclwrite.Tokens
	var n int
	var unresolved []string

	for _, tok := range tokens {
		if tok.Type != hclsyntax.TokenNumberLit {
			out = append(out, tok)
			continue
		}
		id := string(tok.Bytes)
		// Kion ids are positive, so a literal 0 never names a record: it means
		// "none". `parent_ou_id = 0` is the case that matters -- it creates a
		// top-level OU, and a reference there would invent a parent. Left
		// alone, and NOT reported as a boundary gap, because nothing is
		// missing.
		if id == "0" {
			out = append(out, tok)
			continue
		}
		label, ok := byID[id]
		if !ok {
			out = append(out, tok)
			unresolved = append(unresolved, id)
			continue
		}
		out = append(out, traversalTokens(target, label)...)
		n++
	}
	return out, n, unresolved
}

// traversalTokens builds the tokens for `<tfType>.<label>.id`.
func traversalTokens(tfType, label string) hclwrite.Tokens {
	ident := func(s string) *hclwrite.Token {
		return &hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(s)}
	}
	dot := func() *hclwrite.Token {
		return &hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")}
	}
	return hclwrite.Tokens{ident(tfType), dot(), ident(label), dot(), ident("id")}
}
