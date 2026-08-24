package ou_note_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionOuNoteDataSource_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	dataSourceName := "data.kion_ou_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOuNoteDataSourceConfig_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
				),
			},
		},
	})
}

func testAccOuNoteDataSourceConfig_basic() string {
	return `
resource "kion_ou_note" "test" {
  create_user_id = 1
  name           = "test-acc-ou-note-ds"
  ou_id          = 1
  text           = "test-acc-value"
}

data "kion_ou_note" "test" {
  id = kion_ou_note.test.id
}
`
}
