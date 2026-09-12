package ami_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAmiDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// kion_ami takes Kion's own account id, not the cloud account number. The
	// fixture used to hardcode account_id = 1, which is not an account on every
	// install: "Bad Request: account not found".
	accountID := acctest.RequireEnv(t, "KION_ACC_ACCOUNT_ID",
		"the Kion id of an account this install can register an AMI against")
	// Kion assumes its service role in that account and calls DescribeImages, so
	// the image has to exist: an invented id fails with "Could not validate the
	// presence of this AMI". Any AMI the account can describe works, including a
	// public Amazon one -- but ids are per-region and rotate, so it is supplied.
	amiID := acctest.RequireEnv(t, "KION_ACC_AWS_AMI_ID",
		"an AMI id in us-east-1 the install's accounts can describe")

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_ami.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAmiDataSourceConfig_basic(rName, accountID, amiID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "account_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "aws_ami_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "name"),
					resource.TestCheckResourceAttrSet(dataSourceName, "region"),
				),
			},
		},
	})
}

func testAccAmiDataSourceConfig_basic(rName, accountID, amiID string) string {
	return fmt.Sprintf(`
resource "kion_ami" "test" {
  account_id = %[2]s
  aws_ami_id = %[3]q
  name = %[1]q
  region = "us-east-1"
  owner_user_ids = [1]
}

data "kion_ami" "test" {
  id = kion_ami.test.id
}
`, rName, accountID, amiID)
}
