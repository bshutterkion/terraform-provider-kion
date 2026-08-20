package aws_account

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_aws_account", &resource.Sweeper{
		Name: "kion_aws_account",
		F:    sweepAwsAccount,
	})
}

func sweepAwsAccount(_ string) error {
	// TODO: List all kion_aws_account resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}
