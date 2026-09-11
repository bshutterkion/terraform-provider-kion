package ami_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAmiDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_AWS_ACCOUNT_NUMBER") == "" {
		t.Skip("KION_ACC_AWS_ACCOUNT_NUMBER must be set to an AWS account on this install; kion_ami registers an image against one")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.kion_ami.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAmiDataSourceConfig_basic(rName),
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

func testAccAmiDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_ami" "test" {
  account_id = 1
  aws_ami_id = "ami-00000000000000000"
  name = %[1]q
  region = "us-east-1"
  owner_user_ids = [1]
}

data "kion_ami" "test" {
  id = kion_ami.test.id
}
`, rName)
}
