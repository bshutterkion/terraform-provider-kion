package gcp_service_account_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionGcpServiceAccount_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL") == "" {
		t.Skip("KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL must be set to the email of a real GCP service account; kion_gcp_service_account records a Google-issued identity that cannot be invented")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_gcp_service_account.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGcpServiceAccountDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccGcpServiceAccountConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGcpServiceAccountExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "email"),
					resource.TestCheckResourceAttrSet(resourceName, "enable_federation_support"),
					resource.TestCheckResourceAttrSet(resourceName, "gcp_project_id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "unique_id"),
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

func TestAccKionGcpServiceAccount_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	if os.Getenv("KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL") == "" {
		t.Skip("KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL must be set to the email of a real GCP service account; kion_gcp_service_account records a Google-issued identity that cannot be invented")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_gcp_service_account.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGcpServiceAccountDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccGcpServiceAccountConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGcpServiceAccountExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccGcpServiceAccountConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGcpServiceAccountExists(ctx, resourceName),
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

func testAccCheckGcpServiceAccountExists(_ context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID set for %s", name)
		}
		// TODO: Call SDK to verify the resource exists.
		return nil
	}
}

func testAccCheckGcpServiceAccountDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_gcp_service_account" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccGcpServiceAccountConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_gcp_service_account" "test" {
  email = "test-acc-value"
  enable_federation_support = false
  gcp_project_id = "test-acc-value"
  name = %[1]q
  unique_id = "test-acc-value"
}
`, rName)
}

func testAccGcpServiceAccountConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_gcp_service_account" "test" {
  email = "test-acc-updated"
  enable_federation_support = true
  gcp_project_id = "test-acc-updated"
  name = %[1]q
  unique_id = "test-acc-updated"
  description = "test-acc-updated"
}
`, rName)
}
