package app_config

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_app_config", &resource.Sweeper{
		Name: "kion_app_config",
		F:    sweepAppConfig,
	})
}

func sweepAppConfig(_ string) error {
	// TODO: List all kion_app_config resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}
