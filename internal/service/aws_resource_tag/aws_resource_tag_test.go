package aws_resource_tag_test

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

func TestAccKionAwsResourceTag_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_aws_resource_tag.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAwsResourceTagDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAwsResourceTagConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAwsResourceTagExists(ctx, resourceName),
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

// rawLookupAwsResourceTag reports whether the private collection this resource is read
// through still holds the given id. There is no single-record GET, so the
// collection is the only way to see the record.
func rawLookupAwsResourceTag(id string) (bool, error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}

	want, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parsing ID %q: %w", id, err)
	}

	path := "/v3/aws-resource-tag"
	body, err := conn.RawGet(context.Background(), path)
	if err != nil {
		if conns.IsRawNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading %s: %w", path, err)
	}

	var env struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("decoding %s: %w", path, err)
	}

	for _, rec := range env.Data {
		if rec.ID != want {
			continue
		}
		return true, nil
	}
	return false, nil
}

func testAccCheckAwsResourceTagExists(_ context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID set for %s", name)
		}

		found, err := rawLookupAwsResourceTag(rs.Primary.ID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_aws_resource_tag (%s) not found", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckAwsResourceTagDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_aws_resource_tag" {
				continue
			}

			found, err := rawLookupAwsResourceTag(rs.Primary.ID)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("kion_aws_resource_tag (%s) still exists", rs.Primary.ID)
			}
		}

		return nil
	}
}

func testAccAwsResourceTagConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_aws_resource_tag" "test" {
  resource_key = "test-acc-%[1]s"
  resource_value = "test-acc-value"
}
`, rName)
}
