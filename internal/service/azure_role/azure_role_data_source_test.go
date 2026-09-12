package azure_role_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAzureRoleDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_AZURE_PAYER_ID") == "" {
		t.Skip("KION_ACC_AZURE_PAYER_ID must be set to an Azure billing source on this install; POST /v3/azure-role returns 500 without one")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_azure_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureRoleDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "name"),
					resource.TestCheckResourceAttrSet(dataSourceName, "role_permissions"),
				),
			},
		},
	})
}

func testAccAzureRoleDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_azure_role" "test" {
  name = %[1]q
  // Kion parses this as Azure's Permissions OBJECT (or a full role definition
  // with properties.permissions). A JSON ARRAY of permission objects fails to
  // unmarshal and surfaces as a bare 500 with no message.
  role_permissions = jsonencode({ actions = ["Microsoft.Resources/subscriptions/read"], notActions = [] })
  owner_user_ids = [1]
}

data "kion_azure_role" "test" {
  id = kion_azure_role.test.id
}
`, rName)
}
