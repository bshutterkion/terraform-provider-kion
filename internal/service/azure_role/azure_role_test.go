package azure_role_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
	"terraform-provider-kion/internal/errs"

	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"
)

func TestAccKionAzureRole_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_AZURE_PAYER_ID") == "" {
		t.Skip("KION_ACC_AZURE_PAYER_ID must be set to an Azure billing source on this install; POST /v3/azure-role returns 500 without one")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_azure_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureRoleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAzureRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAzureRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "role_permissions"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// An import has no prior value to compare against, so it takes
				// Kion's canonical form of the permissions -- the same document,
				// with the two empty arrays it fills in. ImportStateVerify
				// compares raw strings and so reports that as a difference.
				// Managed state converges (the apply and refresh steps above
				// prove it), and a configuration generated from an import is
				// already canonical, so it plans clean.
				ImportStateVerifyIgnore: []string{"role_permissions"},
			},
		},
	})
}

func TestAccKionAzureRole_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_AZURE_PAYER_ID") == "" {
		t.Skip("KION_ACC_AZURE_PAYER_ID must be set to an Azure billing source on this install; POST /v3/azure-role returns 500 without one")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_azure_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAzureRoleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAzureRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAzureRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAzureRoleConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAzureRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// An import has no prior value to compare against, so it takes
				// Kion's canonical form of the permissions -- the same document,
				// with the two empty arrays it fills in. ImportStateVerify
				// compares raw strings and so reports that as a difference.
				// Managed state converges (the apply and refresh steps above
				// prove it), and a configuration generated from an import is
				// already canonical, so it plans clean.
				ImportStateVerifyIgnore: []string{"role_permissions"},
			},
		},
	})
}

func testAccCheckAzureRoleExists(_ context.Context, name string) resource.TestCheckFunc {
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
		out, err := conn.Client.GetAzureRole(ctx, generated.GetAzureRoleParams{ID: id})
		if err != nil {
			return fmt.Errorf("reading kion_azure_role (%d): %w", id, err)
		}
		if errs.IsNotFound(out) {
			return fmt.Errorf("kion_azure_role (%d) not found", id)
		}

		return nil
	}
}

func testAccCheckAzureRoleDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := acctest.SharedClient()
		if err != nil {
			return fmt.Errorf("getting shared client: %w", err)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_azure_role" {
				continue
			}

			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing ID: %w", err)
			}

			ctx := context.Background()
			out, err := conn.Client.GetAzureRole(ctx, generated.GetAzureRoleParams{ID: id})
			if errs.IsNotFound(out) {
				continue
			}
			if err != nil {
				return fmt.Errorf("reading kion_azure_role (%d): %w", id, err)
			}

			return fmt.Errorf("kion_azure_role (%d) still exists", id)
		}

		return nil
	}
}

func testAccAzureRoleConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_azure_role" "test" {
  name = %[1]q
  // Kion parses this as Azure's Permissions OBJECT (or a full role definition
  // with properties.permissions). A JSON ARRAY of permission objects fails to
  // unmarshal and surfaces as a bare 500 with no message.
  role_permissions = jsonencode({ actions = ["Microsoft.Resources/subscriptions/read"], notActions = [] })
  owner_user_ids = [1]
}
`, rName)
}

func testAccAzureRoleConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_azure_role" "test" {
  name = %[1]q
  role_permissions = jsonencode({ actions = ["Microsoft.Resources/subscriptions/resourceGroups/read"], notActions = [] })
  car_restricted_user_group_ids = []
  owner_user_ids = [1]
}
`, rName)
}
