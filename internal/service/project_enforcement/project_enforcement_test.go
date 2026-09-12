package project_enforcement_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

// testAccProjectEnforcementImportID builds the "project_id/id" import id the
// resource's ImportState requires: an enforcement has no by-id GET, so it is
// addressed through its parent project.
func testAccProjectEnforcementImportID(name string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("not found: %s", name)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["project_id"], rs.Primary.ID), nil
	}
}

func TestAccKionProjectEnforcement_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectEnforcementConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccProjectEnforcementImportID(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccKionProjectEnforcement_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectEnforcementConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccProjectEnforcementConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccProjectEnforcementImportID(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccProjectEnforcementConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test_project" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]
}

resource "kion_project_enforcement" "test" {
  project_id     = kion_project.test_project.id
  description    = "test-acc project enforcement"
  threshold      = 1000
  threshold_type = "dollar"
  timeframe      = "month"
  amount_type    = "custom"
  spend_option   = "spend"
  enabled        = true
  user_ids       = [1]
}
`, rName)
}

func testAccProjectEnforcementConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test_project" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]
}

resource "kion_project_enforcement" "test" {
  project_id     = kion_project.test_project.id
  description    = "test-acc project enforcement updated"
  threshold      = 2000
  threshold_type = "dollar"
  timeframe      = "month"
  amount_type    = "custom"
  spend_option   = "spend"
  enabled        = true
  user_ids       = [1]
}
`, rName)
}

// user_group_ids is renamed from the API's ugroup_ids, and a renamed attribute
// used to stop matching its request body field, so the value was accepted and
// discarded (#94). Neither the basic nor the update config sets it, so nothing
// here covered it. Asserts the attribute survives a round trip.
func TestAccKionProjectEnforcement_userGroupIds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectEnforcementConfigUserGroup(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "user_group_ids.#", "1"),
					resource.TestCheckResourceAttrPair(
						resourceName, "user_group_ids.0", "kion_user_group.test_group", "id"),
				),
			},
		},
	})
}

func testAccProjectEnforcementConfigUserGroup(rName string) string {
	return fmt.Sprintf(`
resource "kion_user_group" "test_group" {
  idms_id        = 1
  name           = "%[1]s-group"
  owner_user_ids = [1]
}

resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test_project" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]
}

resource "kion_project_enforcement" "test" {
  project_id     = kion_project.test_project.id
  description    = "test-acc project enforcement"
  threshold      = 1000
  threshold_type = "dollar"
  timeframe      = "month"
  amount_type    = "custom"
  spend_option   = "spend"
  enabled        = true
  user_ids       = [1]
  user_group_ids = [kion_user_group.test_group.id]
}
`, rName)
}
