package compliance_check_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionComplianceCheckDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_compliance_check.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccComplianceCheckDataSourceConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
				),
			},
		},
	})
}

func testAccComplianceCheckDataSourceConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_compliance_check" "test" {
  name                     = %[1]q
  description              = "test-acc compliance check"
  cloud_provider_id        = 1
  compliance_check_type_id = 1
  severity_type_id         = 3
  owner_user_ids           = [1]
}

data "kion_compliance_check" "test" {
  id = kion_compliance_check.test.id
}
`, rName)
}
