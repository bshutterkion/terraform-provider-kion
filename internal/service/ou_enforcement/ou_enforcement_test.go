// Known issues this test is expected to surface:
//   #70 cloud_rule_id and service_id are dropped on read (the record carries both as nested objects and flatten never unwraps them), so the _update test's ImportStateVerify fails. Left failing on purpose.

package ou_enforcement_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionOuEnforcement_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOuEnforcementDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccOuEnforcementConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuEnforcementExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "ou_id"),
					resource.TestCheckResourceAttrSet(resourceName, "threshold"),
					resource.TestCheckResourceAttrSet(resourceName, "timeframe"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("not found: %s", resourceName)
					}
					return rs.Primary.Attributes["ou_id"] + "/" + rs.Primary.ID, nil
				},
			},
		},
	})
}

func TestAccKionOuEnforcement_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOuEnforcementDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccOuEnforcementConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuEnforcementExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccOuEnforcementConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuEnforcementExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("not found: %s", resourceName)
					}
					return rs.Primary.Attributes["ou_id"] + "/" + rs.Primary.ID, nil
				},
			},
		},
	})
}

func testAccCheckOuEnforcementExists(_ context.Context, name string) resource.TestCheckFunc {
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

func testAccCheckOuEnforcementDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_ou_enforcement" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccOuEnforcementConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_ou" "test_ou" {
  name = "test-acc-ou-%[1]s"
  owner_user_ids = [1]
  parent_ou_id = 0
  permission_scheme_id = 2
}

resource "kion_ou_enforcement" "test" {
  ou_id = kion_ou.test_ou.id
  threshold = 100
  timeframe = "month"
  user_ids = [1]
}
`, rName)
}

func testAccOuEnforcementConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_ou" "test_ou" {
  name = "test-acc-ou-%[1]s"
  owner_user_ids = [1]
  parent_ou_id = 0
  permission_scheme_id = 2
}

resource "kion_ou_enforcement" "test" {
  ou_id = kion_ou.test_ou.id
  threshold = 200
  timeframe = "month"
  cloud_rule_id = 2
  user_ids = [1]
}
`, rName)
}
