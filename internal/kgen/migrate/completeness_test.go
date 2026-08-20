package migrate

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// knownGaps records shared resources whose old→new migration is NOT fully
// automated, each with the reason. Deliberately surfaced (not silently dropped)
// so the story is honest. Empty means every shared resource is fully handled.
// A resource here is exempt from the completeness guards below.
var knownGaps = map[string]string{}

// aliasResources are the back-compat aliases the new provider kept (old name →
// new primary type). Their resource struct embeds the primary, so the upgrader
// generated on the primary (keyed by newType with old_type set) is inherited by
// the alias and runs for old-name state. Config migrates via kmigrate's old-type
// matching. Verified end-to-end by the harness.
var aliasResources = map[string]string{
	"kion_aws_iam_policy":              "iam_policy", // → kion_iam_policy
	"kion_aws_cloudformation_template": "cft",        // → kion_cft
}

// TestAliasResources_haveGeneratedUpgraders asserts each alias's migration is
// closed: a transform is keyed by old_type = the alias, and the primary package
// carries a generated state upgrader (inherited by the alias via embedding).
func TestAliasResources_haveGeneratedUpgraders(t *testing.T) {
	ups, err := LoadUpgrades("../../../codegen/state_upgrades.yaml")
	if err != nil {
		t.Fatal(err)
	}
	oldTypeCovered := map[string]bool{}
	for _, tr := range ups {
		if tr.OldType != "" {
			oldTypeCovered[tr.OldType] = true
		}
	}
	for aliasType, pkg := range aliasResources {
		if !oldTypeCovered[aliasType] {
			t.Errorf("%s: no transform declares old_type: %s — its state won't migrate", aliasType, aliasType)
		}
		if _, err := os.Stat("../../service/" + pkg + "/" + pkg + "_upgrade_gen.go"); err != nil {
			t.Errorf("%s: expected generated upgrader internal/service/%s/%s_upgrade_gen.go (inherited by the alias): %v", aliasType, pkg, pkg, err)
		}
	}
}

// ownershipBlocks are the old-provider block names that became id-list
// attributes in the new schema. Any resource that has one of these as a block
// MUST project it (state_upgrades.yaml) or be a knownGap — otherwise its owners
// silently fail to migrate. This is the guard that the missing aws_iam_policy
// projection tripped.
var ownershipBlocks = []string{"owner_users", "owner_groups", "owner_user_groups", "users", "user_groups"}

// TestCompleteness_ownershipBlocksAreProjected fails if any resource carries an
// old ownership block with no projection covering it and no documented gap.
func TestCompleteness_ownershipBlocksAreProjected(t *testing.T) {
	oldS, err := LoadSchema(oldSnap)
	if err != nil {
		t.Fatal(err)
	}
	ups, err := LoadUpgrades("../../../codegen/state_upgrades.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// A transform may be keyed by the new primary type with old_type = the old
	// type it migrates (alias resources); index those so old-type ownership
	// blocks count as covered.
	byOldType := map[string]Transform{}
	for _, tr := range ups {
		if tr.OldType != "" {
			byOldType[tr.OldType] = tr
		}
	}

	for rt, oldR := range oldS {
		if _, gapped := knownGaps[rt]; gapped {
			continue
		}
		tr, hasYAML := ups[rt]
		if !hasYAML {
			tr, hasYAML = byOldType[rt]
		}
		for _, blk := range ownershipBlocks {
			a, ok := oldR.Attrs[blk]
			if !ok || a.Kind != "block" {
				continue
			}
			// This resource has an ownership block. Require a projection whose
			// source is that block.
			covered := false
			if hasYAML {
				for _, pr := range tr.Project {
					if pr.From == blk {
						covered = true
						break
					}
				}
			}
			if !covered {
				t.Errorf("%s has ownership block %q but no projection covers it (and it is not a knownGap) — owners would fail to migrate", rt, blk)
			}
		}
	}
}

// TestCompleteness_settableDropsAreDocumented fails if a resource drops an old
// SETTABLE attribute (one a customer could have in .tf) that the new schema no
// longer accepts, unless that drop is documented. A dropped settable attribute
// makes the customer's config invalid until removed, so kmigrate must strip it
// or the migration guide must call it out. Dropped computed-only attributes are
// ignored — they never appear in customer config.
func TestCompleteness_settableDropsAreDocumented(t *testing.T) {
	oldS, err := LoadSchema(oldSnap)
	if err != nil {
		t.Fatal(err)
	}
	newS, err := LoadSchema(newSnap)
	if err != nil {
		t.Fatal(err)
	}

	// Reviewed golden: settable old attributes removed from the new schema, per
	// resource. kmigrate's drop pass (or the migration guide) must handle these.
	// Adding a resource/attr here is the explicit acknowledgement of a config
	// break; an unexpected new entry fails the test until reviewed.
	// Exact settable-dropped sets, computed from the schema snapshots. Most are
	// `last_updated` — an old optional-computed field customers rarely wrote and
	// which kmigrate's drop pass strips. `system_managed_policy` (gcp_iam_role)
	// and the account fields are genuine settable removals. The four *_account
	// resources migrate via Layer 3 import (see docs/MIGRATION-COVERAGE.md), so
	// their large drops are handled by re-import, not config rewrite.
	expected := map[string][]string{
		"kion_app_config":                  {"forecasting_enabled", "idms_groups_as_viewers_default", "saml_debug"},
		"kion_aws_cloudformation_template": {"last_updated"},
		"kion_azure_account":               {"labels", "last_updated", "name", "parent_management_group_id", "subscription_name"},
		"kion_azure_arm_template":          {"last_updated"},
		"kion_azure_policy":                {"description", "last_updated", "name", "parameters", "policy"},
		"kion_azure_role":                  {"last_updated"},
		"kion_custom_account":              {"account_type_id", "labels", "last_updated", "name", "skip_access_checking"},
		"kion_funding_source":              {"labels", "last_updated"},
		"kion_gcp_account":                 {"create_mode", "google_cloud_parent_name", "labels", "last_updated", "name"},
		"kion_gcp_iam_role":                {"last_updated", "system_managed_policy"},
		"kion_project_note":                {"last_updated"},
		"kion_service_control_policy":      {"last_updated"},
	}

	for rt, oldR := range oldS {
		newR, ok := newS[rt]
		if !ok {
			continue
		}
		var dropped []string
		for name, a := range oldR.Attrs {
			if a.Kind != "attr" || !a.Settable() {
				continue
			}
			if _, still := newR.Attrs[name]; !still {
				dropped = append(dropped, name)
			}
		}
		sort.Strings(dropped)
		want := expected[rt]
		sort.Strings(want)
		if !eqStrs(dropped, want) {
			t.Errorf("%s settable-dropped attrs = %v, reviewed golden = %v — update the golden (and kmigrate/docs) after reviewing the config break", rt, dropped, want)
		}
	}
}

// TestConfigDropsAreSettableAndRemoved keeps kmigrate's ConfigDrops honest:
// every attribute it strips must be an old settable attribute genuinely absent
// from the new schema (dropping a still-valid attribute would corrupt config).
func TestConfigDropsAreSettableAndRemoved(t *testing.T) {
	oldS, err := LoadSchema(oldSnap)
	if err != nil {
		t.Fatal(err)
	}
	newS, err := LoadSchema(newSnap)
	if err != nil {
		t.Fatal(err)
	}
	for rt, drops := range ConfigDrops {
		oldR, ok := oldS[rt]
		if !ok {
			t.Errorf("ConfigDrops has %s which is not an old resource", rt)
			continue
		}
		newR := newS[rt]
		for _, a := range drops {
			oa, ok := oldR.Attrs[a]
			if !ok || !oa.Settable() {
				t.Errorf("%s: ConfigDrops %q is not a settable old attribute", rt, a)
			}
			if _, still := newR.Attrs[a]; still {
				t.Errorf("%s: ConfigDrops %q still exists in the new schema — dropping it would corrupt valid config", rt, a)
			}
		}
	}
}

// TestReadOnlyDropsArePresentAndComputed is the guard behind ReadOnlyDrops, both
// ways round. Forwards: every entry must be an old settable attribute that the
// new schema still declares but only as computed — if it were absent it belongs
// in ConfigDrops, and if it were still settable dropping it would corrupt valid
// config. Backwards: every settable→read-only change in the snapshots must have
// an entry, or kmigrate leaves config Terraform rejects with "Invalid
// Configuration for Read-Only Attribute".
//
// `id` is excluded from the backwards half: SDKv2 injects an optional+computed
// top-level `id` into every resource schema, so all 33 old resources report it
// as settable when no provider code ever declared it (see ReadOnlyDrops).
func TestReadOnlyDropsArePresentAndComputed(t *testing.T) {
	oldS, err := LoadSchema(oldSnap)
	if err != nil {
		t.Fatal(err)
	}
	newS, err := LoadSchema(newSnap)
	if err != nil {
		t.Fatal(err)
	}

	readOnly := func(a Attr) bool { return a.Kind == "attr" && a.Computed && !a.Settable() }

	// Forwards: each entry is real.
	for rt, drops := range ReadOnlyDrops {
		oldR, ok := oldS[rt]
		if !ok {
			t.Errorf("ReadOnlyDrops has %s which is not an old resource", rt)
			continue
		}
		newR, ok := newS[rt]
		if !ok {
			t.Errorf("ReadOnlyDrops has %s which is not a new resource", rt)
			continue
		}
		for _, a := range drops {
			oa, ok := oldR.Attrs[a]
			if !ok || oa.Kind != "attr" || !oa.Settable() {
				t.Errorf("%s: ReadOnlyDrops %q is not a settable old attribute", rt, a)
			}
			na, ok := newR.Attrs[a]
			if !ok {
				t.Errorf("%s: ReadOnlyDrops %q is absent from the new schema — it belongs in ConfigDrops", rt, a)
				continue
			}
			if !readOnly(na) {
				t.Errorf("%s: ReadOnlyDrops %q is still settable in the new schema — dropping it would corrupt valid config", rt, a)
			}
		}
	}

	// Backwards: no settable → read-only change is missing an entry.
	for rt, oldR := range oldS {
		newR, ok := newS[rt]
		if !ok {
			continue
		}
		for name, oa := range oldR.Attrs {
			if name == "id" || oa.Kind != "attr" || !oa.Settable() {
				continue
			}
			na, ok := newR.Attrs[name]
			if !ok || !readOnly(na) {
				continue
			}
			if !contains(ReadOnlyDrops[rt], name) {
				t.Errorf("%s.%s: settable in old, read-only in new, but ReadOnlyDrops has no entry — "+
					"kmigrate would leave config Terraform rejects", rt, name)
			}
		}
	}
}

// TestMapToObjectList_matchSchemas is the guard behind MapToObjectList, both
// ways round: every entry must really be an old map that became a list of
// objects with exactly the named fields, and every such change in the snapshots
// must have an entry. A wrong field name here writes config that fails to
// decode; a missing entry silently leaves a map where the provider wants a list.
func TestMapToObjectList_matchSchemas(t *testing.T) {
	oldS, err := LoadSchema(oldSnap)
	if err != nil {
		t.Fatal(err)
	}
	newS, err := LoadSchema(newSnap)
	if err != nil {
		t.Fatal(err)
	}
	ups, err := LoadUpgrades("../../../codegen/state_upgrades.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// The table is keyed by the type as written in old config, which for an alias
	// is the old name; its schema lives under the new primary type.
	newTypeOf := map[string]string{}
	for newType, tr := range ups {
		if tr.OldType != "" {
			newTypeOf[tr.OldType] = newType
		}
	}
	newAttrs := func(rtype, attr string) (Attr, bool) {
		for _, key := range []string{rtype, newTypeOf[rtype]} {
			if r, ok := newS[key]; ok {
				if a, ok := r.Attrs[attr]; ok {
					return a, true
				}
			}
		}
		return Attr{}, false
	}

	// Forwards: each entry is real.
	for rtype, attrs := range MapToObjectList {
		oldR, ok := oldS[rtype]
		if !ok {
			t.Errorf("MapToObjectList has %s which is not an old resource", rtype)
			continue
		}
		for attr, fold := range attrs {
			oa, ok := oldR.Attrs[attr]
			if !ok || !isMapType(oa) {
				t.Errorf("%s: MapToObjectList %q is not an old map attribute", rtype, attr)
			}
			na, ok := newAttrs(rtype, attr)
			if !ok || !na.NestedObj {
				t.Errorf("%s: MapToObjectList %q is not a nested-object attribute in the new schema", rtype, attr)
				continue
			}
			want := []string{fold.KeyField, fold.ValField}
			sort.Strings(want)
			if !eqStrs(na.NestedAttrs, want) {
				t.Errorf("%s.%s: new nested fields = %v, MapToObjectList names %v", rtype, attr, na.NestedAttrs, want)
			}
		}
	}

	// Backwards: no map→object-list change is missing an entry.
	for rtype, oldR := range oldS {
		for attr, oa := range oldR.Attrs {
			if !isMapType(oa) {
				continue
			}
			na, ok := newAttrs(rtype, attr)
			if !ok || !na.NestedObj {
				continue
			}
			if _, covered := MapToObjectList[rtype][attr]; !covered {
				t.Errorf("%s.%s: map became a nested-object attribute %v but MapToObjectList has no entry — kmigrate would leave an invalid map",
					rtype, attr, na.NestedAttrs)
			}
		}
	}
}

// isMapType reports whether an attribute's cty type is a map (the JSON form is
// `["map", <element>]`).
func isMapType(a Attr) bool {
	return a.Kind == "attr" && strings.HasPrefix(a.TypeJSON, `["map"`)
}

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
