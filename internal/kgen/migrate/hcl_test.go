package migrate

import (
	"strings"
	"testing"
)

// TestRewriteFile_dropFoldBlockAttr covers the three non-projection transforms:
// dropping an obsolete attribute (ConfigDrops), folding scattered attributes
// into a nested object (AttrsToObject, azure_policy), and converting a
// nested-object block into a list attribute (BlockToListAttr, project_funding).
func TestRewriteFile_dropFoldBlockAttr(t *testing.T) {
	src := `resource "kion_azure_policy" "p" {
  name        = "p1"
  description = "d"
  policy      = "{}"
  parameters  = "{}"
  last_updated = "x"
}

resource "kion_project" "pr" {
  name = "proj"
  project_funding {
    amount            = 100
    funding_source_id = 7
  }
}
`
	out, changes, _, err := RewriteFile([]byte(src), map[string]Transform{})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	ng := norm(got)

	// azure_policy: body folded, last_updated dropped.
	if !strings.Contains(ng, "azure_policy = {") {
		t.Errorf("azure_policy object not created:\n%s", got)
	}
	if strings.Contains(got, "last_updated") {
		t.Errorf("last_updated not dropped:\n%s", got)
	}
	for _, f := range []string{"name", "description", "policy", "parameters"} {
		if !strings.Contains(ng, f+" =") {
			t.Errorf("folded field %q missing:\n%s", f, got)
		}
	}
	// project_funding block → list attribute.
	if !strings.Contains(ng, "project_funding = [{") {
		t.Errorf("project_funding not converted to list attr:\n%s", got)
	}
	if strings.Contains(got, "project_funding {") {
		t.Errorf("project_funding block not removed:\n%s", got)
	}
	if len(changes) == 0 {
		t.Error("no changes reported")
	}
}

func TestRewriteFile_projectAndRename(t *testing.T) {
	ups := map[string]Transform{
		"kion_user_group": {
			Project: map[string]ProjectRule{
				"owner_user_ids": {From: "owner_users", Field: "id"},
				"user_ids":       {From: "users", Field: "id"},
			},
		},
		"kion_azure_account": {
			Rename: map[string]string{"name": "account_name"},
		},
	}

	src := `resource "kion_user_group" "team" {
  name = "platform"

  owner_users {
    id = 5
  }
  owner_users {
    id = kion_user.admin.id
  }
  users {
    id = 9
  }
}

resource "kion_azure_account" "prod" {
  name       = "prod"
  payer_id   = 3
}
`
	out, changes, _, err := RewriteFile([]byte(src), ups)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	for _, want := range []string{
		"owner_user_ids = [5, kion_user.admin.id]",
		"user_ids       = [9]", // hclwrite may align; check the value form loosely below
		"account_name = \"prod\"",
	} {
		// allow alignment whitespace differences on the "=" side
		norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
		if !strings.Contains(norm(got), norm(want)) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "owner_users {") || strings.Contains(got, "users {") {
		t.Errorf("source blocks not removed:\n%s", got)
	}
	// the azure_account block must no longer declare a bare `name` attribute
	azBlock := got[strings.Index(got, `"kion_azure_account"`):]
	if strings.Contains(azBlock, "\n  name ") || strings.Contains(azBlock, "\n  name=") {
		t.Errorf("azure_account name not renamed:\n%s", got)
	}
	if len(changes) == 0 {
		t.Error("no changes reported")
	}
}
