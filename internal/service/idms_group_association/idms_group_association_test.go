// Hand-written rather than generated: the test has to interpolate an
// install-specific SAML IDMS id into its HCL, which the kgen tests registry's
// RequiredEnv guards cannot do (they gate, they do not thread values). Follows
// internal/service/billing_rule, the existing precedent for that shape.
//
// Known issues this test may surface:
//
//	#48 GET /v3/idms/{id}/group-association 502s for every IDMS type except
//	SAML, on every install.
package idms_group_association_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

// samlIDMSID returns the SAML IDMS to associate against, or skips. A group
// association can only be created under a SAML IDMS: POST answers
// "Bad Request: saml idms not found" for an internal-directory or AD IDMS, so
// the test cannot stand up its own parent.
func samlIDMSID(t *testing.T) string {
	t.Helper()

	id := os.Getenv("KION_ACC_SAML_IDMS_ID")
	if id == "" {
		t.Skip("KION_ACC_SAML_IDMS_ID must be set to the ID of a SAML IDMS on this install; " +
			"group associations cannot be created under any other IDMS type")
	}
	return id
}

func TestAccKionIdmsGroupAssociation_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	idmsID := samlIDMSID(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsGroupAssociationConfigBasic(rName, idmsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "assertion_name", rName),
					resource.TestCheckResourceAttr(resourceName, "idms_id", idmsID),
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

func TestAccKionIdmsGroupAssociation_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	idmsID := samlIDMSID(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsGroupAssociationConfigBasic(rName, idmsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "assertion_regex", "^test-acc$"),
				),
			},
			{
				Config: testAccIdmsGroupAssociationConfigUpdate(rName, idmsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "assertion_regex", "^test-acc-upd$"),
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

func testAccIdmsGroupAssociationConfigBasic(rName, idmsID string) string {
	return fmt.Sprintf(`
resource "kion_idms_group_association" "test" {
  idms_id         = %[2]s
  user_group_id   = 1
  assertion_name  = %[1]q
  assertion_regex = "^test-acc$"
  update_on_login = false
}
`, rName, idmsID)
}

func testAccIdmsGroupAssociationConfigUpdate(rName, idmsID string) string {
	return fmt.Sprintf(`
resource "kion_idms_group_association" "test" {
  idms_id         = %[2]s
  user_group_id   = 1
  assertion_name  = %[1]q
  assertion_regex = "^test-acc-upd$"
  update_on_login = true
}
`, rName, idmsID)
}
