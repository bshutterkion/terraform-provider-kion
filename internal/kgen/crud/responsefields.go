package crud

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FieldPolicyPath is the hand-maintained list of response fields the data
// source must NOT expose, relative to the project root.
const FieldPolicyPath = "codegen/unexposed_fields.yaml"

// FieldPolicy decides which response-only scalars a list data source exposes.
//
// A list data source used to expose the intersection of the create request
// body and the read response, because buildListDSData walked the resource's
// attributes and looked each up in the response. Fields the server owns and a
// client cannot set were therefore unreachable from a configuration: no
// `filter` block could match on them and they were absent from the `list`
// objects. Provenance flags are the fields that suffer most -- ct_managed,
// system_managed_policy and their siblings are exactly what a create body has
// no reason to carry, and exactly what tells a shipped record from a
// customer's.
//
// The default is now to expose every renderable response scalar, and this
// policy is the exception list. Inverting it that way matters: the previous
// behavior hid new fields silently, whereas hiding one now takes an entry in
// a reviewed file. Credentials are the entries that must never be removed --
// account_cache's key_id and key_secret are live secrets, and they were kept
// out of the schema only by the accident this change undoes.
type FieldPolicy struct {
	hidden map[string]bool
}

// LoadFieldPolicy reads FieldPolicyPath. A missing file is an error rather
// than an empty policy: silently exposing credentials because a file went
// missing is the one failure mode worth being loud about.
func LoadFieldPolicy(root string) (FieldPolicy, error) {
	path := filepath.Join(root, FieldPolicyPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return FieldPolicy{}, fmt.Errorf("reading %s: %w", FieldPolicyPath, err)
	}
	var entries map[string]string
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return FieldPolicy{}, fmt.Errorf("parsing %s: %w", FieldPolicyPath, err)
	}
	hidden := make(map[string]bool, len(entries))
	for key := range entries {
		hidden[key] = true
	}
	return FieldPolicy{hidden: hidden}, nil
}

// Hidden reports whether pkg's field is withheld from the data source.
func (p FieldPolicy) Hidden(pkg, field string) bool {
	return p.hidden[pkg+"."+field]
}
