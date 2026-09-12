// Known issues this test is expected to surface:
//   #77 program_id is Required but no flatten assigns it, so an imported control carries none and ImportStateVerify fails. It is also never sent on create: DELETE /v4/compliance/program/{id}/control/{controlID} is the only thing that reads it. Left failing on purpose.
//   #78 arm_template_definition_ids = [] comes back null, because the API stores an empty array as null and flattenComplianceControl uses flex.Uint64SliceToFrameworkSet rather than the OrEmpty variant #67 added. The _update apply is rejected as an inconsistent result. Left failing on purpose.

package compliance_control_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-kion/internal/acctest"
	"terraform-provider-kion/internal/errs"

	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"
)

func TestAccKionComplianceControl_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_compliance_control.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckComplianceControlDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccComplianceControlConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckComplianceControlExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "program_id"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				// program_id is not in the read payload and delete addresses the
				// control through it, so the import id carries both.
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[resourceName]
					return rs.Primary.Attributes["program_id"] + "/" + rs.Primary.ID, nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccKionComplianceControl_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_compliance_control.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckComplianceControlDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccComplianceControlConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckComplianceControlExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccComplianceControlConfig_update(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckComplianceControlExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				// program_id is not in the read payload and delete addresses the
				// control through it, so the import id carries both.
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[resourceName]
					return rs.Primary.Attributes["program_id"] + "/" + rs.Primary.ID, nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckComplianceControlExists(_ context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID set for %s", name)
		}

		conn, err := acctest.SharedClient()
		if err != nil {
			return fmt.Errorf("getting shared client: %w", err)
		}

		id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing ID: %w", err)
		}

		ctx := context.Background()
		out, err := conn.Client.GetComplianceControl(ctx, generated.GetComplianceControlParams{ID: id})
		if err != nil {
			return fmt.Errorf("reading kion_compliance_control (%d): %w", id, err)
		}
		if errs.IsNotFound(out) {
			return fmt.Errorf("kion_compliance_control (%d) not found", id)
		}

		return nil
	}
}

func testAccCheckComplianceControlDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := acctest.SharedClient()
		if err != nil {
			return fmt.Errorf("getting shared client: %w", err)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_compliance_control" {
				continue
			}

			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing ID: %w", err)
			}

			ctx := context.Background()
			out, err := conn.Client.GetComplianceControl(ctx, generated.GetComplianceControlParams{ID: id})
			if errs.IsNotFound(out) {
				continue
			}
			if err != nil {
				return fmt.Errorf("reading kion_compliance_control (%d): %w", id, err)
			}

			return fmt.Errorf("kion_compliance_control (%d) still exists", id)
		}

		return nil
	}
}

func testAccComplianceControlConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_compliance_program" "test_program" {
  name = "test-acc-program-%[1]s"
  version = "1.0"
}

resource "kion_compliance_family" "test_family" {
  compliance_program_id = kion_compliance_program.test_program.id
  name = "test-acc-family-%[1]s"
}

resource "kion_compliance_control" "test" {
  program_id = kion_compliance_program.test_program.id
  compliance_family_id = kion_compliance_family.test_family.id
  name = %[1]q
  description = "test-acc control"
  control_number = 1
  severity = "low"
  title = "test-acc control title"
}
`, rName)
}

func testAccComplianceControlConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_compliance_program" "test_program" {
  name = "test-acc-program-%[1]s"
  version = "1.0"
}

resource "kion_compliance_family" "test_family" {
  compliance_program_id = kion_compliance_program.test_program.id
  name = "test-acc-family-%[1]s"
}

resource "kion_compliance_control" "test" {
  program_id = kion_compliance_program.test_program.id
  arm_template_definition_ids = []
  compliance_family_id = kion_compliance_family.test_family.id
  name = %[1]q
  description = "test-acc control"
  control_number = 1
  severity = "low"
  title = "test-acc control title"
}
`, rName)
}
