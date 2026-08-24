package billing_rule_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionBillingRule_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	billingSourceID := os.Getenv("KION_ACC_BILLING_SOURCE_ID")
	if billingSourceID == "" {
		t.Skip("KION_ACC_BILLING_SOURCE_ID must be set to the ID of an existing Kion billing source")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_billing_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBillingRuleConfigBasic(rName, billingSourceID),
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

func TestAccKionBillingRule_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	billingSourceID := os.Getenv("KION_ACC_BILLING_SOURCE_ID")
	if billingSourceID == "" {
		t.Skip("KION_ACC_BILLING_SOURCE_ID must be set to the ID of an existing Kion billing source")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_billing_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBillingRuleConfigBasic(rName, billingSourceID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccBillingRuleConfigUpdate(rName, billingSourceID),
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

func testAccBillingRuleConfigBasic(rName, billingSourceID string) string {
	return fmt.Sprintf(`
resource "kion_billing_rule" "test" {
  name               = %[1]q
  description        = "test-acc billing rule"
  billing_source_ids = [%[2]s]
  rule_type          = 2
  rule_value         = 5.0
  start_month        = 202501
}
`, rName, billingSourceID)
}

func testAccBillingRuleConfigUpdate(rName, billingSourceID string) string {
	return fmt.Sprintf(`
resource "kion_billing_rule" "test" {
  name               = %[1]q
  description        = "test-acc billing rule updated"
  billing_source_ids = [%[2]s]
  rule_type          = 2
  rule_value         = 10.0
  start_month        = 202501
}
`, rName, billingSourceID)
}
