package project_line_item_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"

	"terraform-provider-kion/internal/acctest"
	"terraform-provider-kion/internal/errs"
)

func TestAccKionProjectLineItem_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// The line item draws on a billing source: category_id and payer_id both
	// resolve to one, and Kion rejects a line item whose payer it cannot find.
	payerID := acctest.RequireEnv(t, "KION_ACC_BILLING_SOURCE_ID",
		"the ID of a billing source on the target Kion; a line item is charged against one")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_line_item.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectLineItemDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectLineItemConfig_basic(rName, payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProjectLineItemExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "amount", "100"),
					resource.TestCheckResourceAttr(resourceName, "description", "test-acc line item"),
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

func TestAccKionProjectLineItem_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	payerID := acctest.RequireEnv(t, "KION_ACC_BILLING_SOURCE_ID",
		"the ID of a billing source on the target Kion; a line item is charged against one")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_line_item.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectLineItemDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectLineItemConfig_basic(rName, payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProjectLineItemExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "amount", "100"),
				),
			},
			{
				Config: testAccProjectLineItemConfig_update(rName, payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProjectLineItemExists(ctx, resourceName),
					// Assert the new values, not merely survival: an update that
					// sends nothing would otherwise pass.
					resource.TestCheckResourceAttr(resourceName, "amount", "250"),
					resource.TestCheckResourceAttr(resourceName, "description", "test-acc line item, updated"),
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

func readProjectLineItem(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	out, err := conn.Client.GetProjectLineItem(ctx, generated.GetProjectLineItemParams{ID: id})
	if err != nil {
		return false, err
	}
	if errs.IsNotFound(out) {
		return false, nil
	}
	return true, nil
}

func stateProjectLineItemID(s *terraform.State, name string) (int64, error) {
	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return 0, fmt.Errorf("not found: %s", name)
	}
	id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing ID %q: %w", rs.Primary.ID, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("%s has ID 0 in state", name)
	}
	return id, nil
}

func testAccCheckProjectLineItemExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateProjectLineItemID(s, name)
		if err != nil {
			return err
		}
		found, err := readProjectLineItem(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_project_line_item (%d) not found", id)
		}
		return nil
	}
}

func testAccCheckProjectLineItemDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_project_line_item" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readProjectLineItem(ctx, id)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("kion_project_line_item (%d) still exists", id)
			}
		}
		return nil
	}
}

// A line item charges a category against a project's funding source, so it needs
// the whole chain: an OU, three permission schemes, a funding source, a project
// with a budget drawn on it, and a category under the same billing source.
func testAccProjectLineItemPrereqs(rName, payerID string) string {
	return acctest.FundingSourceConfig(rName) + fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_fs_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]

  budget = [{
    amount             = 500
    start_datecode     = "2026-01"
    end_datecode       = "2026-12"
    funding_source_ids = [kion_funding_source.test_fs.id]
  }]
}

resource "kion_category" "test" {
  name     = "%[1]s-cat"
  payer_id = %[2]s
}
`, rName, payerID)
}

func testAccProjectLineItemConfig_basic(rName, payerID string) string {
	return testAccProjectLineItemPrereqs(rName, payerID) + fmt.Sprintf(`
resource "kion_project_line_item" "test" {
  project_id        = kion_project.test.id
  category_id       = kion_category.test.id
  funding_source_id = kion_funding_source.test_fs.id
  payer_id          = %[1]s
  amount            = 100
  datecode          = 202601
  description       = "test-acc line item"
}
`, payerID)
}

func testAccProjectLineItemConfig_update(rName, payerID string) string {
	return testAccProjectLineItemPrereqs(rName, payerID) + fmt.Sprintf(`
resource "kion_project_line_item" "test" {
  project_id        = kion_project.test.id
  category_id       = kion_category.test.id
  funding_source_id = kion_funding_source.test_fs.id
  payer_id          = %[1]s
  amount            = 250
  datecode          = 202601
  description       = "test-acc line item, updated"
}
`, payerID)
}
