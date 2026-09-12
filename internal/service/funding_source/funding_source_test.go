package funding_source_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
	"terraform-provider-kion/internal/errs"

	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"
)

func TestAccKionFundingSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFundingSourceDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "amount"),
					resource.TestCheckResourceAttrSet(resourceName, "end_datecode"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "start_datecode"),
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

func TestAccKionFundingSource_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFundingSourceDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccFundingSourceConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceExists(ctx, resourceName),
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

func testAccCheckFundingSourceExists(_ context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID set for %s", name)
		}

		conn, err := acctest.SharedClient()
		if err != nil {
			return fmt.Errorf("getting shared client: %w", err)
		}

		id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing ID: %w", err)
		}

		ctx := context.Background()
		out, err := conn.Client.GetFundingSource(ctx, generated.GetFundingSourceParams{ID: id})
		if err != nil {
			return fmt.Errorf("reading kion_funding_source (%d): %w", id, err)
		}
		if errs.IsNotFound(out) {
			return fmt.Errorf("kion_funding_source (%d) not found", id)
		}

		return nil
	}
}

func testAccCheckFundingSourceDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := acctest.SharedClient()
		if err != nil {
			return fmt.Errorf("getting shared client: %w", err)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_funding_source" {
				continue
			}

			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing ID: %w", err)
			}

			ctx := context.Background()
			out, err := conn.Client.GetFundingSource(ctx, generated.GetFundingSourceParams{ID: id})
			if errs.IsNotFound(out) {
				continue
			}
			if err != nil {
				return fmt.Errorf("reading kion_funding_source (%d): %w", id, err)
			}

			return fmt.Errorf("kion_funding_source (%d) still exists", id)
		}

		return nil
	}
}

func testAccFundingSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

// A funding source rejects an ou-typed scheme: "app policy type not valid for
// this object". The OU above still needs the ou-typed one.
resource "kion_permission_scheme" "test_fs_perm" {
  name = "%[1]s-fs-perm"
  type = "funding_source"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_funding_source" "test" {
  amount               = 1000.00
  end_datecode         = "2026-12"
  name                 = %[1]q
  start_datecode       = "2026-01"
  owner_user_ids       = [1]
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_fs_perm.id
}
`, rName)
}

func testAccFundingSourceConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm" {
  name = "%[1]s-perm"
  type = "ou"
}

// A funding source rejects an ou-typed scheme: "app policy type not valid for
// this object". The OU above still needs the ou-typed one.
resource "kion_permission_scheme" "test_fs_perm" {
  name = "%[1]s-fs-perm"
  type = "funding_source"
}

resource "kion_ou" "test_ou" {
  name                 = "%[1]s-ou"
  parent_ou_id         = 0
  permission_scheme_id = kion_permission_scheme.test_perm.id
  owner_user_ids       = [1]
}

resource "kion_funding_source" "test" {
  amount               = 2000.00
  end_datecode         = "2027-12"
  name                 = %[1]q
  start_datecode       = "2026-01"
  description          = "test-acc-updated"
  owner_user_ids       = [1]
  ou_id                = kion_ou.test_ou.id
  permission_scheme_id = kion_permission_scheme.test_fs_perm.id
}
`, rName)
}
