package project_note_test

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

func TestAccKionProjectNote_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectNoteDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectNoteConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProjectNoteExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
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

func TestAccKionProjectNote_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_note.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectNoteDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectNoteConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProjectNoteExists(ctx, resourceName),
					resource.TestCheckResourceAttr(resourceName, "text", "test-acc project note"),
				),
			},
			{
				Config: testAccProjectNoteConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckProjectNoteExists(ctx, resourceName),
					// The update goes through the PRIVATE /v2 PATCH; the public
					// spec has no by-id update at all. Asserting the new text
					// rather than just "still exists" is what makes a silent
					// no-op update fail.
					resource.TestCheckResourceAttr(resourceName, "text", "test-acc project note, updated"),
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

// readProjectNote fetches a note straight from the API, bypassing the provider.
// The by-id read is private /v2, so there is no SDK method to call: the check
// speaks raw HTTP, as the resource does.
func readProjectNote(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	body, err := conn.RawGet(ctx, "/v2/project-note/"+strconv.FormatInt(id, 10))
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

func stateProjectNoteID(s *terraform.State, name string) (int64, error) {
	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return 0, fmt.Errorf("not found: %s", name)
	}
	// A zero id in state means the create extracted nothing and the record is
	// unaddressable, which reads identically to "deleted" on the next plan.
	// Rejecting it here makes that a test failure rather than a silent pass.
	id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing ID %q: %w", rs.Primary.ID, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("%s has ID 0 in state", name)
	}
	return id, nil
}

func testAccCheckProjectNoteExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateProjectNoteID(s, name)
		if err != nil {
			return err
		}
		found, err := readProjectNote(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("kion_project_note (%d) not found", id)
		}
		return nil
	}
}

func testAccCheckProjectNoteDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_project_note" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			found, err := readProjectNote(ctx, id)
			if err != nil {
				return err
			}
			if found {
				return fmt.Errorf("kion_project_note (%d) still exists", id)
			}
		}
		return nil
	}
}

// A project note needs a project, which needs an OU and a project-typed
// permission scheme. acctest.FundingSourceConfig supplies the OU (and the
// ou-typed scheme it needs); the project hangs off that OU.
func testAccProjectNotePrereqs(rName string) string {
	return acctest.FundingSourceConfig(rName) + fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_fs_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]

  budget = [{
    amount             = 500
    start_datecode     = "2026-01"
    end_datecode       = "2026-12"
    funding_source_ids = [kion_funding_source.test_fs.id]
  }]
}
`, rName)
}

func testAccProjectNoteConfig_basic(rName string) string {
	return testAccProjectNotePrereqs(rName) + fmt.Sprintf(`
resource "kion_project_note" "test" {
  project_id = kion_project.test.id
  # Required by POST /v3/project-note, and settable only because the schema was
  # corrected: it was computed, which made the resource impossible to create.
  create_user_id = 1
  name           = %[1]q
  text           = "test-acc project note"
}
`, rName)
}

func testAccProjectNoteConfig_update(rName string) string {
	return testAccProjectNotePrereqs(rName) + fmt.Sprintf(`
resource "kion_project_note" "test" {
  project_id = kion_project.test.id
  # Required by POST /v3/project-note, and settable only because the schema was
  # corrected: it was computed, which made the resource impossible to create.
  create_user_id = 1
  name           = %[1]q
  text           = "test-acc project note, updated"
}
`, rName)
}
