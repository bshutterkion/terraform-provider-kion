package category_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionCategoryDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_BILLING_SOURCE_ID") == "" {
		t.Skip("KION_ACC_BILLING_SOURCE_ID must be set to a billing source on this install; POST /v3/category requires payer_id")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_category.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCategoryDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "name"),
				),
			},
		},
	})
}

func testAccCategoryDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_category" "test" {
  name = %[1]q
}

data "kion_category" "test" {
  id = kion_category.test.id
}
`, rName)
}
