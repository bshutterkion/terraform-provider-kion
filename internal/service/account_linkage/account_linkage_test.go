package account_linkage_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

// payerEnv describes KION_ACC_AZURE_PAYER_ID for the skip message.
//
// It must be an AZURE billing source, not just any one. A linkage row carries a
// foreign key to `payer`, so on an install with no billing source every create
// fails on the constraint itself:
//
//	Cannot add or update a child row: a foreign key constraint fails
//	(`cloudtamer`.`user_azure_object_id`, CONSTRAINT `f_payer_id` …)
//
// but a NON-Azure payer is worse than that, because it looks like it worked.
// POST answers 201 with a record id, and the by-id read then answers 404 -- the
// read only serves linkages under an Azure payer, so the record exists and is
// unreachable. Against KION_ACC_PAYER_ID (the generic one) that surfaced as
// "Not Found: The linkage you requested could not be found" on a resource the
// apply had just created. The resource is Azure-specific: every attribute it
// carries besides the two ids is an Azure identity.
const payerEnv = "the ID of an AZURE billing source on the target Kion; a linkage under any other payer is created and then cannot be read back"

func TestAccKionAccountLinkage_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	payerID := acctest.RequireEnv(t, "KION_ACC_AZURE_PAYER_ID", payerEnv)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	ctx := acctest.Context(t)
	resourceName := "kion_account_linkage.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccountLinkageDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLinkageConfig_basic(rName, payerID),
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
				// Kion stores a principal name split into azure_username and
				// azure_domain and never returns it as given, so an import
				// cannot recover it and a configuration generated from one has
				// to supply it. Reconstructing username@domain would not help:
				// it does not equal what was configured unless the practitioner
				// wrote a fully-qualified name to begin with.
				ImportStateVerifyIgnore: []string{"azure_principal_name"},
			},
		},
	})
}

func TestAccKionAccountLinkage_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	payerID := acctest.RequireEnv(t, "KION_ACC_AZURE_PAYER_ID", payerEnv)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	ctx := acctest.Context(t)
	resourceName := "kion_account_linkage.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAccountLinkageDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLinkageConfig_basic(rName, payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAccountLinkageConfig_update(rName, payerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLinkageExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// Kion stores a principal name split into azure_username and
				// azure_domain and never returns it as given, so an import
				// cannot recover it and a configuration generated from one has
				// to supply it. Reconstructing username@domain would not help:
				// it does not equal what was configured unless the practitioner
				// wrote a fully-qualified name to begin with.
				ImportStateVerifyIgnore: []string{"azure_principal_name"},
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

// The Azure identity is randomized per run. Kion rejects a second linkage for
// an identity already linked -- "Bad Request: This Azure account is already
// linked" -- so a fixed value passes once and then fails on every rerun until
// the leftover is deleted by hand.
func testAccAccountLinkageConfig_basic(rName, payerID string) string {
	return fmt.Sprintf(`
resource "kion_account_linkage" "test" {
  azure_object_id = %[2]q
  azure_principal_name = %[2]q
  payer_id = %[1]s
  user_id = 1
}
`, payerID, rName, rName+"-updated")
}

// The update step changes the Azure identity rather than payer_id or user_id:
// both name records the install has to already hold, and only one of each is
// known to exist.
func testAccAccountLinkageConfig_update(rName, payerID string) string {
	return fmt.Sprintf(`
resource "kion_account_linkage" "test" {
  azure_object_id = %[3]q
  azure_principal_name = %[3]q
  payer_id = %[1]s
  user_id = 1
}
`, payerID, rName, rName+"-updated")
}
