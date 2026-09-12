package ami_test

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

func TestAccKionAmi_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// kion_ami takes Kion's own account id, not the cloud account number. The
	// fixture used to hardcode account_id = 1, which is not an account on every
	// install: "Bad Request: account not found".
	accountID := acctest.RequireEnv(t, "KION_ACC_ACCOUNT_ID",
		"the Kion id of an account this install can register an AMI against")
	// Kion assumes its service role in that account and calls DescribeImages, so
	// the image has to exist: an invented id fails with "Could not validate the
	// presence of this AMI". Any AMI the account can describe works, including a
	// public Amazon one -- but ids are per-region and rotate, so it is supplied.
	amiID := acctest.RequireEnv(t, "KION_ACC_AWS_AMI_ID",
		"an AMI id in us-east-1 the install's accounts can describe")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ami.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAmiDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAmiConfig_basic(rName, accountID, amiID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAmiExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "account_id"),
					resource.TestCheckResourceAttrSet(resourceName, "aws_ami_id"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "region"),
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

func TestAccKionAmi_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// kion_ami takes Kion's own account id, not the cloud account number. The
	// fixture used to hardcode account_id = 1, which is not an account on every
	// install: "Bad Request: account not found".
	accountID := acctest.RequireEnv(t, "KION_ACC_ACCOUNT_ID",
		"the Kion id of an account this install can register an AMI against")
	// Kion assumes its service role in that account and calls DescribeImages, so
	// the image has to exist: an invented id fails with "Could not validate the
	// presence of this AMI". Any AMI the account can describe works, including a
	// public Amazon one -- but ids are per-region and rotate, so it is supplied.
	amiID := acctest.RequireEnv(t, "KION_ACC_AWS_AMI_ID",
		"an AMI id in us-east-1 the install's accounts can describe")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ami.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAmiDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAmiConfig_basic(rName, accountID, amiID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAmiExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAmiConfig_update(rName, accountID, amiID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAmiExists(ctx, resourceName),
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

func testAccCheckAmiExists(_ context.Context, name string) resource.TestCheckFunc {
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
		out, err := conn.Client.GetAMI(ctx, generated.GetAMIParams{ID: id})
		if err != nil {
			return fmt.Errorf("reading kion_ami (%d): %w", id, err)
		}
		if errs.IsNotFound(out) {
			return fmt.Errorf("kion_ami (%d) not found", id)
		}

		return nil
	}
}

func testAccCheckAmiDestroy(_ context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := acctest.SharedClient()
		if err != nil {
			return fmt.Errorf("getting shared client: %w", err)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "kion_ami" {
				continue
			}

			id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing ID: %w", err)
			}

			ctx := context.Background()
			out, err := conn.Client.GetAMI(ctx, generated.GetAMIParams{ID: id})
			if errs.IsNotFound(out) {
				continue
			}
			if err != nil {
				return fmt.Errorf("reading kion_ami (%d): %w", id, err)
			}

			return fmt.Errorf("kion_ami (%d) still exists", id)
		}

		return nil
	}
}

func testAccAmiConfig_basic(rName, accountID, amiID string) string {
	return fmt.Sprintf(`
resource "kion_ami" "test" {
  account_id = %[2]s
  aws_ami_id = %[3]q
  name = %[1]q
  region = "us-east-1"
  owner_user_ids = [1]
}
`, rName, accountID, amiID)
}

func testAccAmiConfig_update(rName, accountID, amiID string) string {
	return fmt.Sprintf(`
resource "kion_ami" "test" {
  account_id = %[2]s
  aws_ami_id = %[3]q
  name = %[1]q
  region = "us-east-1"
  description = "test-acc-updated"
  owner_user_ids = [1]
}
`, rName, accountID, amiID)
}
