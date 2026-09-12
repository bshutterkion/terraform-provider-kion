package app_role_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
	"terraform-provider-kion/internal/conns"
)

func TestAccKionAppRole_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_app_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppRoleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAppRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAppRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					// Kion sets this; a role created through the API is never
					// system-managed, and asserting it proves the read reaches
					// the private /v1 route rather than leaving the field null.
					resource.TestCheckResourceAttr(resourceName, "system_managed", "false"),
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

func TestAccKionAppRole_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_app_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppRoleDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAppRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAppRoleExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
				),
			},
			{
				Config: testAccAppRoleConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAppRoleExists(ctx, resourceName),
					// Assert the new value, not merely survival: the update goes
					// through the public PATCH while the read-back comes from the
					// private /v1 route, so a no-op update would otherwise pass.
					resource.TestCheckResourceAttr(resourceName, "name", rName+"-upd"),
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

// readAppRole fetches a role straight from the API. The by-id read is private
// /v1 -- the public spec has the list and the PATCH but no by-id GET -- so the
// check speaks raw HTTP, as the resource does.
func readAppRole(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	body, err := conn.RawGet(ctx, "/v1/app-role/"+strconv.FormatInt(id, 10))
	if err != nil {
		if conns.IsRawNotFound(err) {
			return false, nil
		}
		return false, err
	}
	// A deleted role answers 500, not 404, so the id inside the envelope is the
	// only reliable signal that the record is really there.
	var env struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("decoding response: %w", err)
	}
	return env.Data.ID == id, nil
}

func stateAppRoleID(s *terraform.State, name string) (int64, error) {
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

func testAccCheckAppRoleExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateAppRoleID(s, name)
		if err != nil {
			return err
		}
		found, err := readAppRole(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_app_role (%d) not found", id)
		}
		return nil
	}
}

func testAccCheckAppRoleDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_app_role" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readAppRole(ctx, id)
			if err != nil {
				// A deleted role's read answers 500 rather than 404, so an error
				// here is the expected outcome of a successful destroy.
				continue
			}
			if found {
				return fmt.Errorf("kion_app_role (%d) still exists", id)
			}
		}
		return nil
	}
}

func testAccAppRoleConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_app_role" "test" {
  # name is the only thing Kion accepts: AppRoleCreate and AppRoleUpdate carry
  # nothing else, and disabled is read-only (it was settable and discarded).
  name = %[1]q
}
`, rName)
}

func testAccAppRoleConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_app_role" "test" {
  name = "%[1]s-upd"
}
`, rName)
}
