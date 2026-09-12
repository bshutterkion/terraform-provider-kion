package funding_source_note_test

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

// readNote fetches a note straight from the API, bypassing the provider.
// funding_source_note is served entirely over private /v2, so there is no SDK
// method to call: the check has to speak raw HTTP, as the resource does.
func readNote(ctx context.Context, id int64) (found bool, err error) {
	conn, err := acctest.SharedClient()
	if err != nil {
		return false, fmt.Errorf("getting shared client: %w", err)
	}
	body, err := conn.RawGet(ctx, "/v2/funding-source-note/"+strconv.FormatInt(id, 10))
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

func stateNoteID(s *terraform.State, name string) (int64, error) {
	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return 0, fmt.Errorf("not found: %s", name)
	}
	id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing ID %q: %w", rs.Primary.ID, err)
	}
	// The id this whole resource was broken on. #71 wrote "0" to state for a
	// note that really existed, so an assertion that merely reads state back is
	// not enough: the id has to be one the API answers to.
	if id <= 0 {
		return 0, fmt.Errorf("%s recorded id %d, which is not a usable Kion id", name, id)
	}
	return id, nil
}

func testAccCheckFundingSourceNoteExists(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := stateNoteID(s, name)
		if err != nil {
			return err
		}
		found, err := readNote(ctx, id)
		if err != nil {
			return fmt.Errorf("reading note %d: %w", id, err)
		}
		if !found {
			return fmt.Errorf("note %d is in state but not in Kion", id)
		}
		return nil
	}
}

// testAccCheckFundingSourceNoteDestroy is what proves the leak is fixed: a note
// Terraform destroyed has to be gone from the API, not merely gone from state.
func testAccCheckFundingSourceNoteDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_funding_source_note" {
				continue
			}
			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing ID %q: %w", rs.Primary.ID, err)
			}
			found, err := readNote(ctx, id)
			if err != nil {
				return fmt.Errorf("reading note %d: %w", id, err)
			}
			if found {
				return fmt.Errorf("note %d still exists in Kion after destroy", id)
			}
		}
		return nil
	}
}

func testAccFundingSourceNoteConfig_basic(rName string) string {
	return acctest.FundingSourceConfig(rName) + `
resource "kion_funding_source_note" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  name = "test-acc-note"
  text = "test-acc note body"
}
`
}

func testAccFundingSourceNoteConfig_update(rName string) string {
	return acctest.FundingSourceConfig(rName) + `
resource "kion_funding_source_note" "test" {
  funding_source_id = kion_funding_source.test_fs.id
  name = "test-acc-note"
  text = "test-acc note body, updated"
}
`
}
