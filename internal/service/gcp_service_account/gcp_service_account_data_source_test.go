package gcp_service_account_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionGcpServiceAccountDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL") == "" {
		t.Skip("KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL must be set to the email of a real GCP service account; kion_gcp_service_account records a Google-issued identity that cannot be invented")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_gcp_service_account.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGcpServiceAccountDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "email"),
					resource.TestCheckResourceAttrSet(dataSourceName, "enable_federation_support"),
					resource.TestCheckResourceAttrSet(dataSourceName, "gcp_project_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "name"),
					resource.TestCheckResourceAttrSet(dataSourceName, "unique_id"),
				),
			},
		},
	})
}

func testAccGcpServiceAccountDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_gcp_service_account" "test" {
  email = "test-acc-value"
  enable_federation_support = false
  gcp_project_id = "test-acc-value"
  name = %[1]q
  unique_id = "test-acc-value"
}

data "kion_gcp_service_account" "test" {
  id = kion_gcp_service_account.test.id
}
`, rName)
}
