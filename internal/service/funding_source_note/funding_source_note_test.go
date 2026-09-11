// Known issues this test is expected to surface:
//   #71 every note is created with id 0: POST /v2/funding-source-note returns {"status":201,"data":""} and Create decodes a record_id that is not there. Refresh then finds nothing and destroy deletes id 0. Left failing on purpose.

package funding_source_note_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionFundingSourceNote_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFundingSourceNoteDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceNoteConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceNoteExists(ctx, resourceName),
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

func TestAccKionFundingSourceNote_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_funding_source_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFundingSourceNoteDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccFundingSourceNoteConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceNoteExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccFundingSourceNoteConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFundingSourceNoteExists(ctx, resourceName),
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

func testAccCheckFundingSourceNoteExists(_ context.Context, name string) resource.TestCheckFunc {
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

func testAccCheckFundingSourceNoteDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_funding_source_note" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccFundingSourceNoteConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_funding_source" "test_fs" {
  amount = 1000.00
  end_datecode = "2026-12"
  name = "test-acc-fs-%[1]s"
  owner_user_ids = [1]
  permission_scheme_id = 4
  start_datecode = "2026-01"
}

resource "kion_funding_source_note" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  name = "test-acc-note"
  text = "test-acc note body"
}
`, rName)
}

func testAccFundingSourceNoteConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_funding_source" "test_fs" {
  amount = 1000.00
  end_datecode = "2026-12"
  name = "test-acc-fs-%[1]s"
  owner_user_ids = [1]
  permission_scheme_id = 4
  start_datecode = "2026-01"
}

resource "kion_funding_source_note" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  name = "test-acc-note"
  text = "test-acc note body"
}
`, rName)
}
