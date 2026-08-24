package azure_account_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAzureAccountDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	payerID := os.Getenv("KION_ACC_AZURE_PAYER_ID")
	if payerID == "" {
		t.Skip("KION_ACC_AZURE_PAYER_ID must be set to the ID of an existing Kion Azure billing source")
	}

	subscriptionUUID := os.Getenv("KION_ACC_AZURE_SUBSCRIPTION_UUID")
	if subscriptionUUID == "" {
		t.Skip("KION_ACC_AZURE_SUBSCRIPTION_UUID must be set to the UUID of an Azure subscription Kion may manage")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_azure_account.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureAccountDataSourceConfig_basic(rName, payerID, subscriptionUUID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
				),
			},
		},
	})
}

func testAccAzureAccountDataSourceConfig_basic(rName, payerID, subscriptionUUID string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test_project" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]
}

resource "kion_azure_account" "test" {
  account_name      = %[1]q
  payer_id          = %[2]s
  project_id        = kion_project.test_project.id
  subscription_uuid = %[3]q
  start_datecode    = "2025-01"
}

data "kion_azure_account" "test" {
  id = kion_azure_account.test.id
}
`, rName, payerID, subscriptionUUID)
}
