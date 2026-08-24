package iam_policy_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionIamPolicy_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_iam_policy.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIamPolicyConfigBasic(rName),
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

func TestAccKionIamPolicy_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_iam_policy.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIamPolicyConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccIamPolicyConfigUpdate(rName),
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

func testAccIamPolicyConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_aws_iam_policy" "test" {
  name           = %[1]q
  description    = "test-acc IAM policy"
  owner_user_ids = [1]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Deny"
      Action   = "s3:*"
      Resource = "*"
    }]
  })
}
`, rName)
}

func testAccIamPolicyConfigUpdate(rName string) string {
	return fmt.Sprintf(`
resource "kion_aws_iam_policy" "test" {
  name           = %[1]q
  description    = "test-acc IAM policy updated"
  owner_user_ids = [1]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Deny"
      Action   = "ec2:*"
      Resource = "*"
    }]
  })
}
`, rName)
}
