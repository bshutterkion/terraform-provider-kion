package custom_variable_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionCustomVariableDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_custom_variable.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomVariableDataSourceConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
				),
			},
		},
	})
}

func testAccCustomVariableDataSourceConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_custom_variable" "test" {
  name                 = %[1]q
  description          = "test-acc custom variable"
  type                 = "string"
  default_value_string = "test-acc-default"
  owner_user_ids       = [1]
}

data "kion_custom_variable" "test" {
  id = kion_custom_variable.test.id
}
`, rName)
}
