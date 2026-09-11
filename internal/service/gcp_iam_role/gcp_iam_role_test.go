package gcp_iam_role_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionGcpIamRole_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_gcp_iam_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGcpIamRoleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccGcpIamRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGcpIamRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
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

func TestAccKionGcpIamRole_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_gcp_iam_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGcpIamRoleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccGcpIamRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGcpIamRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccGcpIamRoleConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGcpIamRoleExists(ctx, resourceName),
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

func testAccCheckGcpIamRoleExists(_ context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID set for %s", name)
		}
		// TODO: Call SDK to verify the resource exists.
		return nil
	}
}

func testAccCheckGcpIamRoleDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_gcp_iam_role" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccGcpIamRoleConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_gcp_iam_role" "test" {
  name = %[1]q
  owner_user_ids = [1]
  role_permissions = ["resourcemanager.projects.get"]
  gcp_role_launch_stage = 2
}
`, rName)
}

func testAccGcpIamRoleConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_gcp_iam_role" "test" {
  name = %[1]q
  car_restricted_user_group_ids = []
  owner_user_ids = [1]
  role_permissions = ["resourcemanager.projects.get"]
  gcp_role_launch_stage = 2
}
`, rName)
}
