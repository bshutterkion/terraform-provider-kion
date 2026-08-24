package azure_policy_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAzurePolicyDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_azure_policy.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzurePolicyDataSourceConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
				),
			},
		},
	})
}

func testAccAzurePolicyDataSourceConfigBasic(rName string) string {
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

data "kion_azure_policy" "test" {
  id = kion_azure_policy.test.id
}
`, rName)
}
