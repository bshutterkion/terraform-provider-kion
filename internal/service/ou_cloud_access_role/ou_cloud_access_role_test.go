package ou_cloud_access_role_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionOuCloudAccessRole_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_cloud_access_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOuCloudAccessRoleConfigBasic(rName),
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

func TestAccKionOuCloudAccessRole_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_cloud_access_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOuCloudAccessRoleConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccOuCloudAccessRoleConfigUpdate(rName),
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

func testAccOuCloudAccessRoleConfigBasic(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_ou_cloud_access_role" "test" {
  name                   = %[1]q
  ou_id                  = kion_ou.test_ou.id
  web_access             = true
  short_term_access_keys = true
  user_ids               = [1]
}
`, rName)
}

func testAccOuCloudAccessRoleConfigUpdate(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_ou_cloud_access_role" "test" {
  name                   = %[1]q
  ou_id                  = kion_ou.test_ou.id
  web_access             = true
  short_term_access_keys = false
  user_ids               = [1]
}
`, rName)
}

// Changing an IAM policy list on an EXISTING role. Create binds these; update
// did not, so the change was accepted and silently discarded and the role kept
// its original policy for ever. The plain _update test does not cover it -- it
// never sets a policy list -- so this asserts the attribute after the change.
func TestAccKionOuCloudAccessRole_policySync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_cloud_access_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOuCloudAccessRoleConfigPolicyOne(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "aws_iam_policies.#", "1"),
				),
			},
			{
				// Swaps the attached policy for a different one. Before the fix
				// this applied cleanly and changed nothing server-side, so the
				// refresh put the original back and the plan never converged.
				Config: testAccOuCloudAccessRoleConfigPolicyTwo(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "aws_iam_policies.#", "1"),
					resource.TestCheckResourceAttrPair(
						resourceName, "aws_iam_policies.0", "kion_iam_policy.two", "id"),
				),
			},
		},
	})
}

func testAccOuCloudAccessRolePolicyPrereqs(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_iam_policy" "one" {
  name           = "%[1]s-p1"
  owner_user_ids = [1]
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Deny", Action = "s3:*", Resource = "*" }]
  })
}

resource "kion_iam_policy" "two" {
  name           = "%[1]s-p2"
  owner_user_ids = [1]
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Deny", Action = "ec2:*", Resource = "*" }]
  })
}
`, rName)
}

func testAccOuCloudAccessRoleConfigPolicyOne(rName string) string {
	return testAccOuCloudAccessRolePolicyPrereqs(rName) + fmt.Sprintf(`
resource "kion_ou_cloud_access_role" "test" {
  name                   = %[1]q
  ou_id                  = kion_ou.test_ou.id
  web_access             = true
  short_term_access_keys = true
  user_ids               = [1]
  aws_iam_policies       = [kion_iam_policy.one.id]
}
`, rName)
}

func testAccOuCloudAccessRoleConfigPolicyTwo(rName string) string {
	return testAccOuCloudAccessRolePolicyPrereqs(rName) + fmt.Sprintf(`
resource "kion_ou_cloud_access_role" "test" {
  name                   = %[1]q
  ou_id                  = kion_ou.test_ou.id
  web_access             = true
  short_term_access_keys = true
  user_ids               = [1]
  aws_iam_policies       = [kion_iam_policy.two.id]
}
`, rName)
}
