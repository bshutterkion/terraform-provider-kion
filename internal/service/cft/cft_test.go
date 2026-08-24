package cft_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionCft_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_cft.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCftConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccKionCft_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_cft.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCftConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccCftConfigUpdate(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCftConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_aws_cloudformation_template" "test" {
  name           = %[1]q
  description    = "test-acc CFT"
  regions        = ["us-east-1"]
  owner_user_ids = [1]

  policy = jsonencode({
    AWSTemplateFormatVersion = "2010-09-09"
    Description              = "test-acc"
    Resources = {
      Topic = {
        Type = "AWS::SNS::Topic"
      }
    }
  })
}
`, rName)
}

func testAccCftConfigUpdate(rName string) string {
	return fmt.Sprintf(`
resource "kion_aws_cloudformation_template" "test" {
  name           = %[1]q
  description    = "test-acc CFT updated"
  regions        = ["us-east-1"]
  owner_user_ids = [1]

  policy = jsonencode({
    AWSTemplateFormatVersion = "2010-09-09"
    Description              = "test-acc"
    Resources = {
      Topic = {
        Type = "AWS::SNS::Topic"
      }
    }
  })
}
`, rName)
}
