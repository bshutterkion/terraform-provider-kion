package ami_test

import (
	"context"
	"fmt"
	"os"
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

	if os.Getenv("KION_ACC_AWS_ACCOUNT_NUMBER") == "" {
		t.Skip("KION_ACC_AWS_ACCOUNT_NUMBER must be set to an AWS account on this install; kion_ami registers an image against one")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ami.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAmiDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAmiConfig_basic(rName),
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

	if os.Getenv("KION_ACC_AWS_ACCOUNT_NUMBER") == "" {
		t.Skip("KION_ACC_AWS_ACCOUNT_NUMBER must be set to an AWS account on this install; kion_ami registers an image against one")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_ami.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAmiDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccAmiConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAmiExists(ctx, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAmiConfig_update(rName),
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

func testAccAmiConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "kion_ami" "test" {
  account_id = 1
  aws_ami_id = "ami-00000000000000000"
  name = %[1]q
  region = "us-east-1"
  owner_user_ids = [1]
}
`, rName)
}

func testAccAmiConfig_update(rName string) string {
	return fmt.Sprintf(`
resource "kion_ami" "test" {
  account_id = 2
  aws_ami_id = "ami-00000000000000000"
  name = %[1]q
  region = "us-east-1"
  description = "test-acc-updated"
  owner_user_ids = [1]
}
`, rName)
}
