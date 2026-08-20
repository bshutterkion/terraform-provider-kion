package project_enforcement

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_project_enforcement", &resource.Sweeper{
		Name: "kion_project_enforcement",
		F:    sweepProjectEnforcement,
	})
}

func sweepProjectEnforcement(_ string) error {
	// TODO: List all kion_project_enforcement resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}
