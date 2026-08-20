package permission_scheme

import (
	"fmt"

	"terraform-provider-kion/internal/conns"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_permission_scheme", &resource.Sweeper{
		Name: "kion_permission_scheme",
		F:    sweepPermissionScheme,
	})
}

func sweepPermissionScheme(_ string) error {
	conn, err := conns.SharedClient()
	if err != nil {
		return fmt.Errorf("getting shared client: %w", err)
	}
	_ = conn

	// TODO: List all kion_permission_scheme resources with "test-acc" prefix and delete them.
	// Use conn.Client to call the appropriate list endpoint,
	// then call conn.Client.DeletePermissionScheme() for each matching resource.
	return nil
}
