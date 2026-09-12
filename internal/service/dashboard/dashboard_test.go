package dashboard_test

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

func TestAccKionDashboard_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_dashboard.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDashboardExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					// Computed audit fields come from the private /v1 read, whose
					// payload does not match the declared shape (config is an
					// object, description a sql.NullString). Asserting one of them
					// proves the read decoded rather than leaving the record bare.
					resource.TestCheckResourceAttrSet(resourceName, "created_by_user_id"),
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

func TestAccKionDashboard_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_dashboard.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccDashboardConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDashboardExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "description", "test-acc dashboard"),
				),
			},
			{
				Config: testAccDashboardConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDashboardExists(ctx, resourceName),
					// Assert the new value, not merely survival: an update that
					// sends nothing would otherwise pass.
					resource.TestCheckResourceAttr(resourceName, "description", "test-acc dashboard, updated"),
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

// readDashboard fetches a dashboard straight from the API. The by-id read is
// private /v1 -- the public /beta route is create-only -- so the check speaks
// raw HTTP, as the resource does.
func readDashboard(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	body, err := conn.RawGet(ctx, "/v1/dashboard/"+strconv.FormatInt(id, 10))
	if err != nil {
		if conns.IsRawNotFound(err) {
			return false, nil
		}
		return false, err
	}
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

func stateDashboardID(s *terraform.State, name string) (int64, error) {
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

func testAccCheckDashboardExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateDashboardID(s, name)
		if err != nil {
			return err
		}
		found, err := readDashboard(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_dashboard (%d) not found", id)
		}
		return nil
	}
}

func testAccCheckDashboardDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_dashboard" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readDashboard(ctx, id)
			if err != nil {
				continue
			}
			if found {
				return fmt.Errorf("kion_dashboard (%d) still exists", id)
			}
		}
		return nil
	}
}

// config is a JSON document carried as a string. is_default is left alone: a
// dashboard that makes itself the default changes global state for every user
// of the install, which a test must not do.
func testAccDashboardConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_dashboard" "test" {
  name        = %[1]q
  description = "test-acc dashboard"
  config      = jsonencode({})
  is_default  = false
}
`, rName)
}

func testAccDashboardConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_dashboard" "test" {
  name        = %[1]q
  description = "test-acc dashboard, updated"
  config      = jsonencode({})
  is_default  = false
}
`, rName)
}
