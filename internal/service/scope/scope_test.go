package scope_test

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

func TestAccKionScope_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_scope.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckScopeDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccScopeConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckScopeExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// The by-id read returns the criteria under criteria_records[],
				// expanded into Kion's own default structure -- a configured {}
				// comes back carrying version, account_criteria and logic. It is
				// not mapped back onto the attribute, so managed state keeps what
				// was configured and stays stable; an import has nothing to start
				// from and leaves it null. A configuration generated from an
				// import therefore has to supply criteria, which the API accepts
				// as absent.
				ImportStateVerifyIgnore: []string{"criteria"},
			},
		},
	})
}

func TestAccKionScope_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_scope.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckScopeDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccScopeConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckScopeExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "description", "test-acc scope"),
				),
			},
			{
				Config: testAccScopeConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckScopeExists(ctx, resourceName),
					// Assert the new value, not merely that the record survived:
					// an update that sends nothing would otherwise pass.
					resource.TestCheckResourceAttr(resourceName, "description", "test-acc scope, updated"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// The by-id read returns the criteria under criteria_records[],
				// expanded into Kion's own default structure -- a configured {}
				// comes back carrying version, account_criteria and logic. It is
				// not mapped back onto the attribute, so managed state keeps what
				// was configured and stays stable; an import has nothing to start
				// from and leaves it null. A configuration generated from an
				// import therefore has to supply criteria, which the API accepts
				// as absent.
				ImportStateVerifyIgnore: []string{"criteria"},
			},
		},
	})
}

func readScope(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	out, err := conn.Client.GetScopeByID(ctx, generated.GetScopeByIDParams{ID: id})
	if err != nil {
		return false, err
	}
	if errs.IsNotFound(out) {
		return false, nil
	}
	return true, nil
}

func stateScopeID(s *terraform.State, name string) (int64, error) {
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

func testAccCheckScopeExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateScopeID(s, name)
		if err != nil {
			return err
		}
		found, err := readScope(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_scope (%d) not found", id)
		}
		return nil
	}
}

func testAccCheckScopeDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_scope" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readScope(ctx, id)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("kion_scope (%d) still exists", id)
			}
		}
		return nil
	}
}

// A scope hangs off a project, which needs an OU, a project-typed permission
// scheme, and a budget drawn on a funding source.
func testAccScopePrereqs(rName string) string {
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
`, rName)
}

// criteria is a JSON OBJECT, not a JSON string. Kion unmarshals it into its own
// criteria record, so a quoted string is rejected with "There is an error in the
// scope criteria format". jsontypes.Normalized holds the document as text and
// the provider sends it as raw JSON, so jsonencode({}) is the right shape.
// scopeAlias truncates to Kion's varchar(16). rName alone is 17 characters, so
// the obvious "<rName>-alias" is rejected -- with a bare 500, before the
// LengthAtMost validator was added to the schema.
func scopeAlias(rName string) string {
	if len(rName) > 16 {
		return rName[:16]
	}
	return rName
}

func testAccScopeConfig_basic(rName string) string {
	return testAccScopePrereqs(rName) + fmt.Sprintf(`
resource "kion_scope" "test" {
  name           = %[1]q
  alias          = %[2]q
  description    = "test-acc scope"
  project_id     = kion_project.test.id
  start_datecode = 202601
  end_datecode   = 202612
  criteria       = jsonencode({})
}
`, rName, scopeAlias(rName))
}

func testAccScopeConfig_update(rName string) string {
	return testAccScopePrereqs(rName) + fmt.Sprintf(`
resource "kion_scope" "test" {
  name           = %[1]q
  alias          = %[2]q
  description    = "test-acc scope, updated"
  project_id     = kion_project.test.id
  start_datecode = 202601
  end_datecode   = 202612
  criteria       = jsonencode({})
}
`, rName, scopeAlias(rName))
}
