package funding_source_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionFundingSourceDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_funding_source.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "end_datecode"),
					resource.TestCheckResourceAttrSet(dataSourceName, "name"),
					resource.TestCheckResourceAttrSet(dataSourceName, "start_datecode"),
				),
			},
		},
	})
}

func testAccFundingSourceDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_funding_source" "test" {
  amount = 1000.00
  end_datecode = "2026-12"
  name = %[1]q
  start_datecode = "2026-01"
  owner_user_ids = [1]
  permission_scheme_id = 4
}

data "kion_funding_source" "test" {
  id = kion_funding_source.test.id
}
`, rName)
}
