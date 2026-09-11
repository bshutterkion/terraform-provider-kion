// Known issues this test is expected to surface:
//   #80 a created exemption is absent from every collection on a 3.16 install — GET /v1/ou/{id}/cloud-access-role-exemption returns [] for a record that POST just returned a record_id for and that DELETE removes cleanly. Read therefore drops the resource from state on every refresh. The existence check reads the same collection Read does, deliberately. Left failing on purpose.

package ou_cloud_access_role_exemption_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
	"terraform-provider-kion/internal/conns"
	"terraform-provider-kion/internal/flex"
)

func TestAccKionOuCloudAccessRoleExemption_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_cloud_access_role_exemption.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOuCloudAccessRoleExemptionDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccOuCloudAccessRoleExemptionConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuCloudAccessRoleExemptionExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("not found: %s", resourceName)
					}
					return rs.Primary.Attributes["ou_id"] + "/" + rs.Primary.ID, nil
				},
			},
		},
	})
}

// rawLookupOuCloudAccessRoleExemption reports whether the private collection this resource is read
// through still holds the given id. There is no single-record GET, so the
// collection is the only way to see the record.
func rawLookupOuCloudAccessRoleExemption(parentID, id string) (bool, error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}

	want, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parsing ID %q: %w", id, err)
	}

	path := strings.Replace("/v1/ou/{parent}/cloud-access-role-exemption", "{parent}", parentID, 1)
	body, err := conn.RawGet(context.Background(), path)
	if err != nil {
		if conns.IsRawNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	var env struct {
		Data []struct {
			ID   int64         `json:"id"`
			Kind *flex.NullInt `json:"ou_cloud_access_role_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("decoding %s: %w", path, err)
	}

	for _, rec := range env.Data {
		if rec.ID != want {
			continue
		}
		// The collection mixes in a neighboring kind's records; only those
		// carrying a valid ou_cloud_access_role_id are this resource.
		if rec.Kind == nil || !rec.Kind.Valid {
			continue
		}
		return true, nil
	}
	return false, nil
}

func testAccCheckOuCloudAccessRoleExemptionExists(_ context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID set for %s", name)
		}

		found, err := rawLookupOuCloudAccessRoleExemption(rs.Primary.Attributes["ou_id"], rs.Primary.ID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_ou_cloud_access_role_exemption (%s) not found", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckOuCloudAccessRoleExemptionDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_ou_cloud_access_role_exemption" {
				continue
			}

			found, err := rawLookupOuCloudAccessRoleExemption(rs.Primary.Attributes["ou_id"], rs.Primary.ID)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("kion_ou_cloud_access_role_exemption (%s) still exists", rs.Primary.ID)
			}
		}

		return nil
	}
}

func testAccOuCloudAccessRoleExemptionConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_ou" "test_ou" {
  name = "test-acc-ou-%[1]s"
  owner_user_ids = [1]
  parent_ou_id = 0
  permission_scheme_id = 2
}

resource "kion_ou_cloud_access_role" "test_car" {
  name = "test-acc-car-%[1]s"
  ou_id = kion_ou.test_ou.id
}

resource "kion_ou_cloud_access_role_exemption" "test" {
  ou_id = kion_ou.test_ou.id
  ou_cloud_access_role_id = kion_ou_cloud_access_role.test_car.id
  reason = "test-acc exemption"
}
`, rName)
}
