package funding_source_permission_mapping_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionFundingSourcePermissionMapping_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := os.Getenv("KION_ACC_APP_ROLE_ID")
	if appRoleID == "" {
		t.Skip("KION_ACC_APP_ROLE_ID must be set to the ID of an app role to map")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourcePermissionMappingConfig_basic(rName, appRoleID),
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

// TestAccKionFundingSourcePermissionMapping_emptyCollection is the regression
// case from issue #65: a configuration that writes an empty member set. Kion
// stores an empty list and reports it back as JSON null, so a read that mapped
// null to a null set left state disagreeing with the configuration on every
// plan.
func TestAccKionFundingSourcePermissionMapping_emptyCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := os.Getenv("KION_ACC_APP_ROLE_ID")
	if appRoleID == "" {
		t.Skip("KION_ACC_APP_ROLE_ID must be set to the ID of an app role to map")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourcePermissionMappingConfig_emptyCollection(rName, appRoleID),
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

func TestAccKionFundingSourcePermissionMapping_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	appRoleID := os.Getenv("KION_ACC_APP_ROLE_ID")
	if appRoleID == "" {
		t.Skip("KION_ACC_APP_ROLE_ID must be set to the ID of an app role to map")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_permission_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourcePermissionMappingConfig_basic(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccFundingSourcePermissionMappingConfig_update(rName, appRoleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "user_groups_ids.#", "1"),
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

// testAccFundingSourcePermissionMappingPrereqs is the funding source the mapping
// hangs off. Its owner_user_ids is what makes a row for app role 1 exist, which
// is the one role assignable under a permission scheme carrying no role
// mappings of its own — Kion silently drops a mapping for any other.
func testAccFundingSourcePermissionMappingPrereqs(rName string) string {
	return acctest.FundingSourceConfig(rName)
}

func testAccFundingSourcePermissionMappingConfig_basic(rName, appRoleID string) string {
	return testAccFundingSourcePermissionMappingPrereqs(rName) + fmt.Sprintf(`
// user_groups_ids is deliberately omitted: an Optional+Computed collection the
// config leaves out is unknown at create, and used to resolve to null while the
// read reported it empty, so the resource never converged.
resource "kion_funding_source_permission_mapping" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  app_role_id       = %[1]s
  user_ids          = [1]
}
`, appRoleID)
}

func testAccFundingSourcePermissionMappingConfig_emptyCollection(rName, appRoleID string) string {
	return testAccFundingSourcePermissionMappingPrereqs(rName) + fmt.Sprintf(`
resource "kion_funding_source_permission_mapping" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  app_role_id       = %[1]s
  user_ids          = [1]
  user_groups_ids   = []
}
`, appRoleID)
}

// The update adds a group rather than emptying user_ids, which would remove the
// funding source's owner server-side and leave kion_funding_source.test_fs at
// odds with its own owner_user_ids.
func testAccFundingSourcePermissionMappingConfig_update(rName, appRoleID string) string {
	return testAccFundingSourcePermissionMappingPrereqs(rName) + fmt.Sprintf(`
resource "kion_user_group" "test_group" {
  idms_id        = 1
  name           = "%[1]s-group"
  description    = "test-acc funding source permission mapping group"
  owner_user_ids = [1]
}

resource "kion_funding_source_permission_mapping" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  app_role_id       = %[2]s
  user_ids          = [1]
  user_groups_ids   = [kion_user_group.test_group.id]
}
`, rName, appRoleID)
}
