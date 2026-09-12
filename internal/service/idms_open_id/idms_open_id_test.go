package idms_open_id_test

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

func TestAccKionIdmsOpenId_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_open_id.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIdmsOpenIdDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsOpenIdConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "client_id", "test-acc-client"),
					resource.TestCheckResourceAttr(resourceName, "scopes.#", "2"),
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

func TestAccKionIdmsOpenId_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_open_id.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIdmsOpenIdDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsOpenIdConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
				),
			},
			{
				Config: testAccIdmsOpenIdConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdExists(ctx, resourceName),
					// Assert the new value, not merely survival.
					resource.TestCheckResourceAttr(resourceName, "name", rName+"-upd"),
					resource.TestCheckResourceAttr(resourceName, "client_id", "test-acc-client-upd"),
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

// readIdmsOpenId reads through the generic IDMS route. An OpenID IDMS is one
// row in the same table, which is also why it deletes through /v3/idms/{id}.
func readIdmsOpenId(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	out, err := conn.Client.GetIDMS(ctx, generated.GetIDMSParams{ID: id})
	if err != nil {
		return false, err
	}
	if errs.IsNotFound(out) {
		return false, nil
	}
	return true, nil
}

func stateIdmsOpenIdID(s *terraform.State, name string) (int64, error) {
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

func testAccCheckIdmsOpenIdExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateIdmsOpenIdID(s, name)
		if err != nil {
			return err
		}
		found, err := readIdmsOpenId(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_idms_open_id (%d) not found", id)
		}
		return nil
	}
}

// testAccCheckIdmsOpenIdDestroy is the assertion that matters most here. The
// resource used to have no delete at all -- the generated Delete was a warning
// saying the IDMS "may still exist in Kion" -- so every destroy left a live
// identity provider behind. /v3/idms/{id} deletes it, and this proves it does.
func testAccCheckIdmsOpenIdDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_idms_open_id" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readIdmsOpenId(ctx, id)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("kion_idms_open_id (%d) still exists", id)
			}
		}
		return nil
	}
}

// The endpoints are deliberately unreachable: Kion stores an OpenID
// configuration without contacting the issuer, so the test needs no live
// identity provider. .invalid is reserved by RFC 2606 and can never resolve.
func testAccIdmsOpenIdConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_idms_open_id" "test" {
  name                   = %[1]q
  client_id              = "test-acc-client"
  issuer                 = "https://test-acc.invalid"
  authorization_endpoint = "https://test-acc.invalid/auth"
  jwks_uri               = "https://test-acc.invalid/jwks"
  scopes                 = ["openid", "email"]
  username_claim         = "sub"
  email_claim            = "email"
  first_name_claim       = "given_name"
  last_name_claim        = "family_name"
  # Required by OpenIDUpdate, though the create accepts a body without it.
  phone_claim            = "phone_number"
}
`, rName)
}

func testAccIdmsOpenIdConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_idms_open_id" "test" {
  name                   = "%[1]s-upd"
  client_id              = "test-acc-client-upd"
  issuer                 = "https://test-acc.invalid"
  authorization_endpoint = "https://test-acc.invalid/auth"
  jwks_uri               = "https://test-acc.invalid/jwks"
  scopes                 = ["openid", "email"]
  username_claim         = "sub"
  email_claim            = "email"
  first_name_claim       = "given_name"
  last_name_claim        = "family_name"
  # Required by OpenIDUpdate, though the create accepts a body without it.
  phone_claim            = "phone_number"
}
`, rName)
}
