package ou_permission_mapping

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_ou_permission_mapping", &resource.Sweeper{
		Name: "kion_ou_permission_mapping",
		F:    sweepOuPermissionMapping,
	})
}

func sweepOuPermissionMapping(_ string) error {
	// TODO: List all kion_ou_permission_mapping resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}
