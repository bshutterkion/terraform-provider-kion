package account

import (
	"fmt"

	"terraform-provider-kion/internal/conns"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_account", &resource.Sweeper{
		Name: "kion_account",
		F:    sweepAccount,
	})
}

func sweepAccount(_ string) error {
	conn, err := conns.SharedClient()
	if err != nil {
		return fmt.Errorf("getting shared client: %w", err)
	}
	_ = conn

	// TODO: List all kion_account resources with "test-acc" prefix and delete them.
	// Use conn.Client to call the appropriate list endpoint,
	// then call conn.Client.DeleteAccount() for each matching resource.
	return nil
}
