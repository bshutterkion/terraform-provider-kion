package migrate

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// BlockToListAttr lists old nested-object blocks that became list/set nested
// ATTRIBUTES in the new schema, so `X { … }` must become `X = [{ … }]`. Unlike
// the ownership id-list projections, these carry a full object body, not just an
// id. (The four *_account resources also have block→single-nested changes but
// migrate via Layer 3 import, not config rewrite, so they are omitted.)
var BlockToListAttr = map[string][]string{
	"kion_project": {"project_funding", "budget", "move_ou_settings"},
}

// objFold folds a set of old top-level scalar attributes into a single new
// nested-object attribute.
type objFold struct {
	Target string   // new nested-object attribute name
	Fields []string // old top-level attributes to move inside it
}

// AttrsToObject lists resources whose old top-level attributes were folded into a
// nested object. kion_azure_policy's name/description/policy/parameters became a
// single `azure_policy { … }` object with those exact fields.
var AttrsToObject = map[string]objFold{
	"kion_azure_policy": {Target: "azure_policy", Fields: []string{"name", "description", "policy", "parameters"}},
}

// RequiredAdditions lists attributes the new schema requires that the old schema
// had no equivalent for. These are not renames — the rewrite handles those — so
// there is no old value to carry over and kmigrate cannot supply one. It reports
// the blocks that need them instead, which is the difference between a config
// that fails `terraform plan` with "Missing required argument" and one the
// practitioner has been told to finish.
//
// TestRequiredAdditions_matchSchemas keeps this in step with the snapshots.
var RequiredAdditions = map[string][]string{
	// The old kion_user had one computed attribute, id, with read and delete and
	// no create: it could only adopt an existing user for deletion. The new
	// resource is full CRUD.
	"kion_user": {"email", "first_name", "idms_id", "last_name", "username"},
}

// RewriteFile applies the input-attribute half of the state_upgrades transforms
// to one .tf file's `resource "kion_*"` blocks: attribute renames and the
// block-set → id-list restructure (owner_users { id = x } → owner_user_ids = [x]).
// It preserves the id expressions verbatim (literals or references) and returns a
// human-readable list of the changes made. Comments and formatting are preserved
// by hclwrite. Computed-only transforms (id_int, unwrap, drops) are state-only
// and never appear in config, so they are ignored here.
//
// The third return is manual follow-ups: things the new schema demands that no
// rewrite can produce. They are kept apart from changes so the CLI does not count
// them as edits it made.
func RewriteFile(src []byte, ups map[string]Transform) ([]byte, []string, []string, error) {
	f, diags := hclwrite.ParseConfig(src, "config.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, nil, nil, fmt.Errorf("parse: %s", diags.Error())
	}
	// Alias resources: customers' config still uses the old type name (e.g.
	// kion_aws_iam_policy), but the transform is keyed by the new primary type.
	// Map each old type to its transform so those blocks get rewritten too.
	byOldType := map[string]Transform{}
	for _, tr := range ups {
		if tr.OldType != "" {
			byOldType[tr.OldType] = tr
		}
	}

	var changes, actions []string
	for _, block := range f.Body().Blocks() {
		if block.Type() != "resource" || len(block.Labels()) < 2 {
			continue
		}
		rtype, rname := block.Labels()[0], block.Labels()[1]
		t, hasTransform := ups[rtype]
		if !hasTransform {
			t, hasTransform = byOldType[rtype]
		}
		_, hasDrops := ConfigDrops[rtype]
		_, hasBlockConv := BlockToListAttr[rtype]
		_, hasFold := AttrsToObject[rtype]
		_, hasRequired := RequiredAdditions[rtype]
		if !hasTransform && !hasDrops && !hasBlockConv && !hasFold && !hasRequired {
			continue
		}
		body := block.Body()

		for oldA, newA := range t.Rename {
			if attr := body.GetAttribute(oldA); attr != nil {
				toks := attr.Expr().BuildTokens(nil)
				body.RemoveAttribute(oldA)
				body.SetAttributeRaw(newA, toks)
				changes = append(changes, fmt.Sprintf("%s.%s: %s → %s", rtype, rname, oldA, newA))
			}
		}

		// Drop obsolete attributes the new schema no longer accepts.
		for _, dropA := range ConfigDrops[rtype] {
			if body.GetAttribute(dropA) != nil {
				body.RemoveAttribute(dropA)
				changes = append(changes, fmt.Sprintf("%s.%s: dropped obsolete attribute %s", rtype, rname, dropA))
			}
		}

		for newA, pr := range t.Project {
			var elems []hclwrite.Tokens
			for _, b := range body.Blocks() {
				if b.Type() != pr.From {
					continue
				}
				if fa := b.Body().GetAttribute(pr.Field); fa != nil {
					elems = append(elems, fa.Expr().BuildTokens(nil))
				}
			}
			if len(elems) == 0 {
				continue
			}
			// Remove the source blocks (collect-then-remove; RemoveBlock is safe
			// per block).
			for _, b := range body.Blocks() {
				if b.Type() == pr.From {
					body.RemoveBlock(b)
				}
			}
			body.SetAttributeRaw(newA, listTokens(elems))
			changes = append(changes, fmt.Sprintf("%s.%s: %s { %s } → %s = [...]", rtype, rname, pr.From, pr.Field, newA))
		}

		// Convert nested-object blocks that became list/set nested attributes:
		// project_funding { … } → project_funding = [{ … }].
		for _, blkName := range BlockToListAttr[rtype] {
			var objs []hclwrite.Tokens
			for _, b := range body.Blocks() {
				if b.Type() == blkName {
					objs = append(objs, blockBodyToObject(b))
				}
			}
			if len(objs) == 0 {
				continue
			}
			for _, b := range body.Blocks() {
				if b.Type() == blkName {
					body.RemoveBlock(b)
				}
			}
			body.SetAttributeRaw(blkName, listTokens(objs))
			changes = append(changes, fmt.Sprintf("%s.%s: %s { … } → %s = [{…}]", rtype, rname, blkName, blkName))
		}

		// Fold scattered top-level attributes into a nested object:
		// azure_policy name/description/policy/parameters → azure_policy = { … }.
		if fold, ok := AttrsToObject[rtype]; ok {
			toks := hclwrite.Tokens{
				{Type: hclsyntax.TokenOBrace, Bytes: []byte("{")},
				{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
			}
			var present []string
			for _, f := range fold.Fields {
				if attr := body.GetAttribute(f); attr != nil {
					toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(f)})
					toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenEqual, Bytes: []byte(" = ")})
					toks = append(toks, attr.Expr().BuildTokens(nil)...)
					toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
					present = append(present, f)
				}
			}
			if len(present) > 0 {
				toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenCBrace, Bytes: []byte("}")})
				for _, f := range present {
					body.RemoveAttribute(f)
				}
				body.SetAttributeRaw(fold.Target, toks)
				changes = append(changes, fmt.Sprintf("%s.%s: %v → %s = {…}", rtype, rname, present, fold.Target))
			}
		}

		// Checked last: a rename above may have just supplied one of these.
		var missing []string
		for _, req := range RequiredAdditions[rtype] {
			if body.GetAttribute(req) == nil {
				missing = append(missing, req)
			}
		}
		if len(missing) > 0 {
			actions = append(actions, fmt.Sprintf(
				"%s.%s: add %v — required by the new schema, and kmigrate has no value to supply",
				rtype, rname, missing))
		}
	}
	return f.Bytes(), changes, actions, nil
}

// blockBodyToObject renders a block's attribute body as an HCL object-constructor
// token sequence: { name = expr\n … }. Nested blocks within the body (rare for
// these funding/settings objects) are not carried over.
func blockBodyToObject(b *hclwrite.Block) hclwrite.Tokens {
	toks := hclwrite.Tokens{
		{Type: hclsyntax.TokenOBrace, Bytes: []byte("{")},
		{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
	}
	names := make([]string, 0, len(b.Body().Attributes()))
	for name := range b.Body().Attributes() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		attr := b.Body().GetAttribute(name)
		toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(name)})
		toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenEqual, Bytes: []byte(" = ")})
		toks = append(toks, attr.Expr().BuildTokens(nil)...)
		toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
	}
	toks = append(toks, &hclwrite.Token{Type: hclsyntax.TokenCBrace, Bytes: []byte("}")})
	return toks
}

// listTokens wraps element expressions into a `[a, b, c]` token sequence.
func listTokens(elems []hclwrite.Tokens) hclwrite.Tokens {
	out := hclwrite.Tokens{{Type: hclsyntax.TokenOBrack, Bytes: []byte("[")}}
	for i, e := range elems {
		if i > 0 {
			out = append(out, &hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(", ")})
		}
		out = append(out, e...)
	}
	out = append(out, &hclwrite.Token{Type: hclsyntax.TokenCBrack, Bytes: []byte("]")})
	return out
}
