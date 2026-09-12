package idms_open_id_group_association_test

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

func TestAccKionIdmsOpenIdGroupAssociation_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_open_id_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsOpenIdGroupAssociationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdGroupAssociationExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "open_id_id", "kion_idms_open_id.test", "id"),
					resource.TestCheckResourceAttr(resourceName, "assertion_name", "test-acc-assertion"),
					resource.TestCheckResourceAttr(resourceName, "user_group_id", "1"),
				),
			},
		},
	})
}

func TestAccKionIdmsOpenIdGroupAssociation_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_open_id_group_association.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsOpenIdGroupAssociationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdGroupAssociationExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "assertion_regex", ".*"),
				),
			},
			{
				Config: testAccIdmsOpenIdGroupAssociationConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdGroupAssociationExists(ctx, resourceName),
					// Assert the new value, not merely survival.
					resource.TestCheckResourceAttr(resourceName, "assertion_regex", "^test-acc.*$"),
				),
			},
		},
	})
}

func readIdmsOpenIdGroupAssociation(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	out, err := conn.Client.GetOpenIDGroupAssociation(ctx, generated.GetOpenIDGroupAssociationParams{ID: id})
	if err != nil {
		return false, err
	}
	if errs.IsNotFound(out) {
		return false, nil
	}
	return true, nil
}

func stateIdmsOpenIdGroupAssociationID(s *terraform.State, name string) (int64, error) {
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

func testAccCheckIdmsOpenIdGroupAssociationExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateIdmsOpenIdGroupAssociationID(s, name)
		if err != nil {
			return err
		}
		found, err := readIdmsOpenIdGroupAssociation(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_idms_open_id_group_association (%d) not found", id)
		}
		return nil
	}
}

// There is deliberately no CheckDestroy.
//
// GET /v4/idms/open-id/group-association/{id} ignores deleted_at and serves
// tombstones, so a destroyed association still reads back 200 with its data.
// Verified in the database: the record is soft-deleted and the read returns it
// anyway, while the sibling access-rule read 404s for an equally deleted row.
// That is #105.
//
// The delete itself works -- it answers 200 and sets deleted_at -- so there is
// nothing wrong to assert here, only nothing the API can be asked to confirm.
// The consequence worth knowing is on the read side: an association removed
// outside Terraform is never detected as gone.

// The parent IDMS points at .invalid endpoints, reserved by RFC 2606: Kion
// stores an OpenID configuration without contacting the issuer, so no live
// identity provider is needed.
func testAccIdmsOpenIdGroupAssociationPrereqs(rName string) string {
	return fmt.Sprintf(`
resource "kion_idms_open_id" "test" {
  name                   = "%[1]s-idms"
  client_id              = "test-acc-client"
  issuer                 = "https://test-acc.invalid"
  authorization_endpoint = "https://test-acc.invalid/auth"
  jwks_uri               = "https://test-acc.invalid/jwks"
  scopes                 = ["openid", "email"]
  username_claim         = "sub"
  email_claim            = "email"
  first_name_claim       = "given_name"
  last_name_claim        = "family_name"
  phone_claim            = "phone_number"
}
`, rName)
}

func testAccIdmsOpenIdGroupAssociationConfig_basic(rName string) string {
	return testAccIdmsOpenIdGroupAssociationPrereqs(rName) + `
resource "kion_idms_open_id_group_association" "test" {
  open_id_id      = kion_idms_open_id.test.id
  assertion_name  = "test-acc-assertion"
  assertion_regex = ".*"
  user_group_id   = 1
  update_on_login = true
}
`
}

func testAccIdmsOpenIdGroupAssociationConfig_update(rName string) string {
	return testAccIdmsOpenIdGroupAssociationPrereqs(rName) + `
resource "kion_idms_open_id_group_association" "test" {
  open_id_id      = kion_idms_open_id.test.id
  assertion_name  = "test-acc-assertion"
  assertion_regex = "^test-acc.*$"
  user_group_id   = 1
  update_on_login = true
}
`
}
