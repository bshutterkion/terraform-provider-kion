package project_permission_mapping_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionProjectPermissionMapping_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := os.Getenv("KION_ACC_APP_ROLE_ID")
	if appRoleID == "" {
		t.Skip("KION_ACC_APP_ROLE_ID must be set to the ID of an app role to map")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectPermissionMappingConfig_basic(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccKionProjectPermissionMapping_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := os.Getenv("KION_ACC_APP_ROLE_ID")
	if appRoleID == "" {
		t.Skip("KION_ACC_APP_ROLE_ID must be set to the ID of an app role to map")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectPermissionMappingConfig_basic(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccProjectPermissionMappingConfig_update(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccKionProjectPermissionMapping_emptyCollection is the regression case
// from issue #65: a configuration that writes an empty member set. Kion stores
// an empty list and reports it back as JSON null, so a read that mapped null to
// a null set left state disagreeing with the configuration on every plan.
func TestAccKionProjectPermissionMapping_emptyCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := os.Getenv("KION_ACC_APP_ROLE_ID")
	if appRoleID == "" {
		t.Skip("KION_ACC_APP_ROLE_ID must be set to the ID of an app role to map")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectPermissionMappingConfig_emptyCollection(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "user_groups_ids.#", "0"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccProjectPermissionMappingConfig_emptyCollection(rName, appRoleID string) string {
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

resource "kion_project_permission_mapping" "test" {
  project_id      = kion_project.test_project.id
  app_role_id     = %[2]s
  user_ids        = [1]
  user_groups_ids = []
}
`, rName, appRoleID)
}

func testAccProjectPermissionMappingConfig_basic(rName, appRoleID string) string {
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

// user_groups_ids is deliberately omitted: an Optional+Computed collection the
// config leaves out is unknown at create, and used to resolve to null while the
// read reported it empty, so the resource never converged.
resource "kion_project_permission_mapping" "test" {
  project_id  = kion_project.test_project.id
  app_role_id = %[2]s
  user_ids    = [1]
}
`, rName, appRoleID)
}

// The update adds a group to user_groups_ids and leaves user_ids alone. It used
// to empty user_ids instead, which removed the project's owner server-side and
// left kion_project.test_project permanently at odds with its own
// owner_user_ids — a conflict between two resources in the same configuration.
func testAccProjectPermissionMappingConfig_update(rName, appRoleID string) string {
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

resource "kion_user_group" "test_group" {
  idms_id        = 1
  name           = "%[1]s-group"
  description    = "test-acc project permission mapping group"
  owner_user_ids = [1]
}

resource "kion_project_permission_mapping" "test" {
  project_id      = kion_project.test_project.id
  app_role_id     = %[2]s
  user_ids        = [1]
  user_groups_ids = [kion_user_group.test_group.id]
}
`, rName, appRoleID)
}
