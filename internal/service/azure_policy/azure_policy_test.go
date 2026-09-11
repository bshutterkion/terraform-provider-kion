package azure_policy_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAzurePolicy_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_azure_policy.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			// Creating either resource makes Kion validate against a live
			// Azure connection; without one the API answers 500, which is an
			// install that cannot run the test rather than a provider defect.
			acctest.RequireEnv(t, "KION_ACC_AZURE_PAYER_ID", "the ID of an Azure billing source on the target Kion")
		},
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzurePolicyConfigBasic(rName),
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

func TestAccKionAzurePolicy_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_azure_policy.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			// Creating either resource makes Kion validate against a live
			// Azure connection; without one the API answers 500, which is an
			// install that cannot run the test rather than a provider defect.
			acctest.RequireEnv(t, "KION_ACC_AZURE_PAYER_ID", "the ID of an Azure billing source on the target Kion")
		},
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzurePolicyConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAzurePolicyConfigUpdate(rName),
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

func testAccAzurePolicyConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_azure_policy" "test" {
  owner_users = [1]

  azure_policy = {
    name        = %[1]q
    description = "test-acc Azure policy"
    policy = jsonencode({
      if = {
        field  = "type"
        equals = "Microsoft.Resources/subscriptions"
      }
      then = {
        effect = "audit"
      }
    })
  }
}
`, rName)
}

func testAccAzurePolicyConfigUpdate(rName string) string {
	return fmt.Sprintf(`
resource "kion_azure_policy" "test" {
  owner_users = [1]

  azure_policy = {
    name        = %[1]q
    description = "test-acc Azure policy updated"
    policy = jsonencode({
      if = {
        field  = "type"
        equals = "Microsoft.Resources/subscriptions"
      }
      then = {
        effect = "audit"
      }
    })
  }
}
`, rName)
}
