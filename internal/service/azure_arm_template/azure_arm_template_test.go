package azure_arm_template_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

func TestAccKionAzureArmTemplate_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	regionID := os.Getenv("KION_ACC_AZURE_REGION_ID")
	if regionID == "" {
		t.Skip("KION_ACC_AZURE_REGION_ID must be set to the Kion database ID of the Azure region to deploy into")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_azure_arm_template.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureArmTemplateConfigBasic(rName, regionID),
				Check: resource.ComposeAggregateTestCheckFunc(
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

func TestAccKionAzureArmTemplate_update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	regionID := os.Getenv("KION_ACC_AZURE_REGION_ID")
	if regionID == "" {
		t.Skip("KION_ACC_AZURE_REGION_ID must be set to the Kion database ID of the Azure region to deploy into")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_azure_arm_template.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAzureArmTemplateConfigBasic(rName, regionID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAzureArmTemplateConfigUpdate(rName, regionID),
				Check: resource.ComposeAggregateTestCheckFunc(
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

func testAccAzureArmTemplateConfigBasic(rName, regionID string) string {
	return fmt.Sprintf(`
resource "kion_azure_arm_template" "test" {
  name                     = %[1]q
  description              = "test-acc ARM template"
  deployment_mode          = 1
  resource_group_name      = "%[1]s-rg"
  resource_group_region_id = %[2]s
  owner_user_ids           = [1]

  template = jsonencode({
    "$schema"      = "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#"
    contentVersion = "1.0.0.0"
    resources      = []
  })
}
`, rName, regionID)
}

func testAccAzureArmTemplateConfigUpdate(rName, regionID string) string {
	return fmt.Sprintf(`
resource "kion_azure_arm_template" "test" {
  name                     = %[1]q
  description              = "test-acc ARM template updated"
  deployment_mode          = 1
  resource_group_name      = "%[1]s-rg"
  resource_group_region_id = %[2]s
  owner_user_ids           = [1]

  template = jsonencode({
    "$schema"      = "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#"
    contentVersion = "1.0.0.0"
    resources      = []
  })
}
`, rName, regionID)
}
