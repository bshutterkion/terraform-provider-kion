package global_permission_mapping_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

// The global mapping is the one association with no parent: it keys straight
// into the install-wide list. The tests therefore write through a user group
// they create themselves and never touch user_ids, so no real person's global
// access is changed even if a run is interrupted. KION_ACC_GLOBAL_APP_ROLE_ID
// must name a non-administrative role for the same reason.
func globalAppRoleID(t *testing.T) string {
	t.Helper()
	id := os.Getenv("KION_ACC_GLOBAL_APP_ROLE_ID")
	if id == "" {
		t.Skip("KION_ACC_GLOBAL_APP_ROLE_ID must be set to a non-administrative app role to map globally")
	}
	return id
}

func TestAccKionGlobalPermissionMapping_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := globalAppRoleID(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_global_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGlobalPermissionMappingConfig_basic(rName, appRoleID),
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

// TestAccKionGlobalPermissionMapping_emptyCollection is the regression case from
// issue #65: a configuration that writes an empty member set. Kion stores an
// empty list and reports it back as JSON null, so a read that mapped null to a
// null set left state disagreeing with the configuration on every plan.
func TestAccKionGlobalPermissionMapping_emptyCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := globalAppRoleID(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_global_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGlobalPermissionMappingConfig_emptyCollection(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "user_ids.#", "0"),
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

func testAccGlobalPermissionMappingGroup(rName string) string {
	return fmt.Sprintf(`
resource "kion_user_group" "test_group" {
  idms_id        = 1
  name           = "%[1]s-group"
  description    = "test-acc global permission mapping group"
  owner_user_ids = [1]
}
`, rName)
}

func testAccGlobalPermissionMappingConfig_basic(rName, appRoleID string) string {
	return testAccGlobalPermissionMappingGroup(rName) + fmt.Sprintf(`
// user_ids is deliberately omitted: an Optional+Computed collection the config
// leaves out is unknown at create, and used to resolve to null while the read
// reported it empty, so the resource never converged.
resource "kion_global_permission_mapping" "test" {
  app_role_id     = %[1]s
  user_groups_ids = [kion_user_group.test_group.id]
}
`, appRoleID)
}

func testAccGlobalPermissionMappingConfig_emptyCollection(rName, appRoleID string) string {
	return testAccGlobalPermissionMappingGroup(rName) + fmt.Sprintf(`
resource "kion_global_permission_mapping" "test" {
  app_role_id     = %[1]s
  user_groups_ids = [kion_user_group.test_group.id]
  user_ids        = []
}
`, appRoleID)
}
