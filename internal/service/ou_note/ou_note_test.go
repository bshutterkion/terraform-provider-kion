package ou_note_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionOuNote_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOuNoteDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccOuNoteConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuNoteExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "create_user_id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "ou_id"),
					resource.TestCheckResourceAttrSet(resourceName, "text"),
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

func TestAccKionOuNote_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ou_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOuNoteDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccOuNoteConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuNoteExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccOuNoteConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckOuNoteExists(ctx, resourceName),
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

func testAccCheckOuNoteExists(_ context.Context, name string) resource.TestCheckFunc {
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

func testAccCheckOuNoteDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_ou_note" {
				continue
			}
			// TODO: Call SDK to verify the resource no longer exists.
			// Return nil if 404, return error if still exists.
		}
		return nil
	}
}

func testAccOuNoteConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_ou_note" "test" {
  create_user_id = 1
  name = %[1]q
  ou_id = 1
  text = "test-acc-value"
}
`, rName)
}

func testAccOuNoteConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_ou_note" "test" {
  create_user_id = 2
  name = %[1]q
  ou_id = 2
  text = "test-acc-updated"
}
`, rName)
}
