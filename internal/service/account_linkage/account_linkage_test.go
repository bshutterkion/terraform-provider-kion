package account_linkage_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

// payerEnv describes KION_ACC_PAYER_ID for the skip message. A linkage row
// carries a foreign key to `payer`, so on an install with no billing source
// every create fails on the constraint itself:
//
//	Cannot add or update a child row: a foreign key constraint fails
//	(`cloudtamer`.`user_azure_object_id`, CONSTRAINT `f_payer_id` …)
//
// That is an install that cannot run the test, not a provider defect (#62).
const payerEnv = "the ID of a billing source on the target Kion; a linkage row references one by foreign key"

func TestAccKionAccountLinkage_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	payerID := acctest.RequireEnv(t, "KION_ACC_PAYER_ID", payerEnv)
	ctx := acctest.Context(t)
	resourceName := "kion_account_linkage.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccountLinkageDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLinkageConfig_basic(payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "azure_object_id"),
					resource.TestCheckResourceAttrSet(resourceName, "azure_principal_name"),
					resource.TestCheckResourceAttrSet(resourceName, "payer_id"),
					resource.TestCheckResourceAttrSet(resourceName, "user_id"),
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

func TestAccKionAccountLinkage_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	payerID := acctest.RequireEnv(t, "KION_ACC_PAYER_ID", payerEnv)
	ctx := acctest.Context(t)
	resourceName := "kion_account_linkage.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccountLinkageDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLinkageConfig_basic(payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAccountLinkageConfig_update(payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
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

func testAccCheckAccountLinkageExists(_ context.Context, name string) resource.TestCheckFunc {
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

func testAccCheckAccountLinkageDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_account_linkage" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccAccountLinkageConfig_basic(payerID string) string {
	return fmt.Sprintf(`
resource "kion_account_linkage" "test" {
  azure_object_id = "test-acc-value"
  azure_principal_name = "test-acc-value"
  payer_id = %[1]s
  user_id = 1
}
`, payerID)
}

// The update step changes the Azure identity rather than payer_id or user_id:
// both name records the install has to already hold, and only one of each is
// known to exist.
func testAccAccountLinkageConfig_update(payerID string) string {
	return fmt.Sprintf(`
resource "kion_account_linkage" "test" {
  azure_object_id = "test-acc-updated"
  azure_principal_name = "test-acc-updated"
  payer_id = %[1]s
  user_id = 1
}
`, payerID)
}
