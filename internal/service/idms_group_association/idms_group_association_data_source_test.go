// Hand-written alongside idms_group_association_test.go; see the note there.
package idms_group_association_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionIdmsGroupAssociationDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	idmsID := samlIDMSID(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_idms_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsGroupAssociationDataSourceConfigBasic(rName, idmsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttr(dataSourceName, "assertion_name", rName),
					resource.TestCheckResourceAttrSet(dataSourceName, "assertion_regex"),
					resource.TestCheckResourceAttrSet(dataSourceName, "idms_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "user_group_id"),
				),
			},
		},
	})
}

func testAccIdmsGroupAssociationDataSourceConfigBasic(rName, idmsID string) string {
	return fmt.Sprintf(`
resource "kion_idms_group_association" "test" {
  idms_id         = %[2]s
  user_group_id   = 1
  assertion_name  = %[1]q
  assertion_regex = "^test-acc$"
  update_on_login = false
}

data "kion_idms_group_association" "test" {
  id = kion_idms_group_association.test.id
}
`, rName, idmsID)
}
