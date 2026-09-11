package user_group_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionUserGroup_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_user_group.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfigBasic(rName),
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

func TestAccKionUserGroup_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_user_group.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccUserGroupConfigUpdate(rName),
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

// owner_user_ids is not decoration: POST /v3/user-group answers "Field
// validation for 'OwnerUserIDs' failed on the 'atLeastOneFieldPresent' tag"
// unless one of owner_user_ids / owner_user_group_ids is present. The schema
// marks both Optional, which is true of each alone but not of the pair (#62).
func testAccUserGroupConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_user_group" "test" {
  idms_id        = 1
  name           = %[1]q
  description    = "test-acc user group"
  owner_user_ids = [1]
}
`, rName)
}

func testAccUserGroupConfigUpdate(rName string) string {
	return fmt.Sprintf(`
resource "kion_user_group" "test" {
  idms_id        = 1
  name           = %[1]q
  description    = "test-acc user group updated"
  owner_user_ids = [1]
}
`, rName)
}
