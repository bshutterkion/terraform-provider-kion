package custom_variable_override

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_custom_variable_override", &resource.Sweeper{
		Name: "kion_custom_variable_override",
		F:    sweepCustomVariableOverride,
	})
}

func sweepCustomVariableOverride(_ string) error {
	// TODO: List all kion_custom_variable_override resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}
