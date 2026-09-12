package project_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionProject_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// No project read returns these: the API's project record is
				// id/name/description/archived/auto_pay/ou_id/default_aws_region
				// on both GET /v3/project/{id} and the list, so they are write-only
				// and cannot survive an import.
				ImportStateVerifyIgnore: []string{"owner_user_ids", "permission_scheme_id"},
			},
		},
	})
}

func TestAccKionProject_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBasic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccProjectConfigUpdate(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// No project read returns these: the API's project record is
				// id/name/description/archived/auto_pay/ou_id/default_aws_region
				// on both GET /v3/project/{id} and the list, so they are write-only
				// and cannot survive an import.
				ImportStateVerifyIgnore: []string{"owner_user_ids", "permission_scheme_id"},
			},
		},
	})
}

// TestAccKionProject_budget covers the budget-mode create path: a project with a
// `budget` block must go to POST /v3/project/with-budget carrying it. The
// spend-plan endpoint refuses the payload outright on a budget-mode install, so
// a wrong dispatch fails the apply rather than passing quietly.
func TestAccKionProject_budget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBudget(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "budget.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "budget.0.amount", "500"),
				),
			},
		},
	})
}

func testAccProjectConfigBudget(rName string) string {
	// The budget carries funding_source_ids. Kion rejects a budget that names
	// neither a funding source nor per-datecode data -- "funding source required
	// if budget data is not provided" -- so without one this failed the create
	// and looked like a bad dispatch rather than an incomplete fixture.
	return acctest.FundingSourceConfig(rName) + fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test" {
  name                 = %[1]q
  ou_id                = kion_ou.test_fs_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]

  # budget is a set of nested attributes, not a block.
  # Within the shared funding source: it holds 1000.00 and runs 2026-01 to
  # 2026-12, and Kion rejects a budget that outspends or outlasts it.
  budget = [{
    amount             = 500
    start_datecode     = "2026-01"
    end_datecode       = "2026-12"
    funding_source_ids = [kion_funding_source.test_fs.id]
  }]
}
`, rName)
}

func testAccProjectConfigBasic(rName string) string {
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

resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test" {
  name                 = %[1]q
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  description          = "test-acc project"
  owner_user_ids       = [1]
}
`, rName)
}

func testAccProjectConfigUpdate(rName string) string {
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

resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test" {
  name                 = %[1]q
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  description          = "test-acc project updated"
  owner_user_ids       = [1]
}
`, rName)
}
