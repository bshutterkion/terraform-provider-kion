package saml_group_association_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionSamlGroupAssociation_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	samlIDMSID := os.Getenv("KION_ACC_SAML_IDMS_ID")
	if samlIDMSID == "" {
		t.Skip("KION_ACC_SAML_IDMS_ID must be set to the ID of a SAML IDMS configured in Kion")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_saml_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSamlGroupAssociationConfig_basic(rName, samlIDMSID),
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

func TestAccKionSamlGroupAssociation_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	samlIDMSID := os.Getenv("KION_ACC_SAML_IDMS_ID")
	if samlIDMSID == "" {
		t.Skip("KION_ACC_SAML_IDMS_ID must be set to the ID of a SAML IDMS configured in Kion")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_saml_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSamlGroupAssociationConfig_basic(rName, samlIDMSID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccSamlGroupAssociationConfig_update(rName, samlIDMSID),
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

func testAccSamlGroupAssociationConfig_basic(rName, samlIDMSID string) string {
	return fmt.Sprintf(`
resource "kion_user_group" "test_group" {
  idms_id = 1
  name    = "%[1]s-group"
}

resource "kion_saml_group_association" "test" {
  idms_id         = %[2]s
  user_group_id   = kion_user_group.test_group.id
  assertion_name  = "memberOf"
  assertion_regex = "^%[1]s$"
  update_on_login = true
}
`, rName, samlIDMSID)
}

func testAccSamlGroupAssociationConfig_update(rName, samlIDMSID string) string {
	return fmt.Sprintf(`
resource "kion_user_group" "test_group" {
  idms_id = 1
  name    = "%[1]s-group"
}

resource "kion_saml_group_association" "test" {
  idms_id         = %[2]s
  user_group_id   = kion_user_group.test_group.id
  assertion_name  = "memberOf"
  assertion_regex = "^%[1]s$"
  update_on_login = false
}
`, rName, samlIDMSID)
}
