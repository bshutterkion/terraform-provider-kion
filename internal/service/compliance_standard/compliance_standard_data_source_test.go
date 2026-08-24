package compliance_standard_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionComplianceStandardDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_compliance_standard.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccComplianceStandardDataSourceConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
				),
			},
		},
	})
}

func testAccComplianceStandardDataSourceConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_compliance_standard" "test" {
  name               = %[1]q
  description        = "test-acc compliance standard"
  created_by_user_id = 1
  owner_user_ids     = [1]
}

data "kion_compliance_standard" "test" {
  id = kion_compliance_standard.test.id
}
`, rName)
}
