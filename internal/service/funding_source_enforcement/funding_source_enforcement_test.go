package funding_source_enforcement_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionFundingSourceEnforcement_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFundingSourceEnforcementDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceEnforcementConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceEnforcementExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "funding_source_id"),
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
					return rs.Primary.Attributes["funding_source_id"] + "/" + rs.Primary.ID, nil
				},
			},
		},
	})
}

func TestAccKionFundingSourceEnforcement_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_enforcement.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFundingSourceEnforcementDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceEnforcementConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceEnforcementExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccFundingSourceEnforcementConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceEnforcementExists(ctx, resourceName),
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
					return rs.Primary.Attributes["funding_source_id"] + "/" + rs.Primary.ID, nil
				},
			},
		},
	})
}

func testAccCheckFundingSourceEnforcementExists(_ context.Context, name string) resource.TestCheckFunc {
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

func testAccCheckFundingSourceEnforcementDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_funding_source_enforcement" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccFundingSourceEnforcementConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_funding_source" "test_fs" {
  amount = 1000.00
  end_datecode = "2026-12"
  name = "test-acc-fs-%[1]s"
  owner_user_ids = [1]
  permission_scheme_id = 4
  start_datecode = "2026-01"
}

resource "kion_funding_source_enforcement" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  threshold = 100
  timeframe = "month"
  user_ids = [1]
}
`, rName)
}

func testAccFundingSourceEnforcementConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_funding_source" "test_fs" {
  amount = 1000.00
  end_datecode = "2026-12"
  name = "test-acc-fs-%[1]s"
  owner_user_ids = [1]
  permission_scheme_id = 4
  start_datecode = "2026-01"
}

resource "kion_funding_source_enforcement" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  threshold = 200
  timeframe = "month"
  cloud_rule_id = 2
  user_ids = [1]
}
`, rName)
}
