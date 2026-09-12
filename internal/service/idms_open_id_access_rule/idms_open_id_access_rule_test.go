package idms_open_id_access_rule_test

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

func TestAccKionIdmsOpenIdAccessRule_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_open_id_access_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIdmsOpenIdAccessRuleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsOpenIdAccessRuleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdAccessRuleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "open_id_id", "kion_idms_open_id.test", "id"),
					resource.TestCheckResourceAttr(resourceName, "assertion_name", "test-acc-assertion"),
					resource.TestCheckResourceAttr(resourceName, "cloudtamer_access_level_id", "1"),
				),
			},
		},
	})
}

func TestAccKionIdmsOpenIdAccessRule_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_idms_open_id_access_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIdmsOpenIdAccessRuleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccIdmsOpenIdAccessRuleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdAccessRuleExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "assertion_regex", ".*"),
				),
			},
			{
				Config: testAccIdmsOpenIdAccessRuleConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIdmsOpenIdAccessRuleExists(ctx, resourceName),
					// Assert the new value, not merely survival.
					resource.TestCheckResourceAttr(resourceName, "assertion_regex", "^test-acc.*$"),
				),
			},
		},
	})
}

func readIdmsOpenIdAccessRule(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	out, err := conn.Client.GetOpenIDAccessRule(ctx, generated.GetOpenIDAccessRuleParams{ID: id})
	if err != nil {
		return false, err
	}
	if errs.IsNotFound(out) {
		return false, nil
	}
	return true, nil
}

func stateIdmsOpenIdAccessRuleID(s *terraform.State, name string) (int64, error) {
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

func testAccCheckIdmsOpenIdAccessRuleExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateIdmsOpenIdAccessRuleID(s, name)
		if err != nil {
			return err
		}
		found, err := readIdmsOpenIdAccessRule(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_idms_open_id_access_rule (%d) not found", id)
		}
		return nil
	}
}

func testAccCheckIdmsOpenIdAccessRuleDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_idms_open_id_access_rule" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readIdmsOpenIdAccessRule(ctx, id)
			if err != nil {
				// The parent IDMS is destroyed alongside, so a read of its child
				// can fail rather than answer 404. Either way it is gone.
				continue
			}
			if found {
				return fmt.Errorf("kion_idms_open_id_access_rule (%d) still exists", id)
			}
		}
		return nil
	}
}

// The parent IDMS points at .invalid endpoints, reserved by RFC 2606: Kion
// stores an OpenID configuration without contacting the issuer, so no live
// identity provider is needed.
func testAccIdmsOpenIdAccessRulePrereqs(rName string) string {
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

func testAccIdmsOpenIdAccessRuleConfig_basic(rName string) string {
	return testAccIdmsOpenIdAccessRulePrereqs(rName) + `
resource "kion_idms_open_id_access_rule" "test" {
  open_id_id      = kion_idms_open_id.test.id
  assertion_name  = "test-acc-assertion"
  assertion_regex = ".*"
  cloudtamer_access_level_id = 1
}
`
}

func testAccIdmsOpenIdAccessRuleConfig_update(rName string) string {
	return testAccIdmsOpenIdAccessRulePrereqs(rName) + `
resource "kion_idms_open_id_access_rule" "test" {
  open_id_id      = kion_idms_open_id.test.id
  assertion_name  = "test-acc-assertion"
  assertion_regex = "^test-acc.*$"
  cloudtamer_access_level_id = 1
}
`
}
