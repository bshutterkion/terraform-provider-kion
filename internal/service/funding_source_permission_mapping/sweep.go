package funding_source_permission_mapping

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_funding_source_permission_mapping", &resource.Sweeper{
		Name: "kion_funding_source_permission_mapping",
		F:    sweepFundingSourcePermissionMapping,
	})
}

func sweepFundingSourcePermissionMapping(_ string) error {
	// TODO: List all kion_funding_source_permission_mapping resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}
