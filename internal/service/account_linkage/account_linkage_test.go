package account_linkage_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAccountLinkage_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	resourceName := "kion_account_linkage.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccountLinkageDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLinkageConfig_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "azure_object_id"),
					resource.TestCheckResourceAttrSet(resourceName, "azure_principal_name"),
					resource.TestCheckResourceAttrSet(resourceName, "payer_id"),
					resource.TestCheckResourceAttrSet(resourceName, "user_id"),
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

func TestAccKionAccountLinkage_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	resourceName := "kion_account_linkage.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccountLinkageDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLinkageConfig_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAccountLinkageConfig_update(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
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

func testAccCheckAccountLinkageExists(_ context.Context, name string) resource.TestCheckFunc {
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

func testAccCheckAccountLinkageDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_account_linkage" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccAccountLinkageConfig_basic() string {
	return `
resource "kion_account_linkage" "test" {
  azure_object_id = "test-acc-value"
  azure_principal_name = "test-acc-value"
  payer_id = 1
  user_id = 1
}
`
}

func testAccAccountLinkageConfig_update() string {
	return `
resource "kion_account_linkage" "test" {
  azure_object_id = "test-acc-updated"
  azure_principal_name = "test-acc-updated"
  payer_id = 2
  user_id = 2
  id = 2
}
`
}
