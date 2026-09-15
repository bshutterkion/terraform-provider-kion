package automation_policy

import (
	"context"

	"terraform-provider-kion/internal/conns"
	"terraform-provider-kion/internal/servicepkg"
)

var _ conns.ServicePackage = &servicePackage{}

type servicePackage struct{}

// NewServicePackage returns the service package registration.
func NewServicePackage() conns.ServicePackage {
	return &servicePackage{}
}

func (p *servicePackage) Resources(_ context.Context) []servicepkg.ServicePackageResource {
	return []servicepkg.ServicePackageResource{
		{
			Factory: NewAutomationPolicyResource,
		},
	}
}

// No data source is registered for this service package. The endpoint supports
// one -- GET /v1/automation-policy lists every policy -- so this is a gap to
// close, not a limit of the API. It is left out rather than shipped as the
// scaffold generated it, which registered a `kion_automation_policy` data
// source whose Read was a TODO and would have returned nothing.
func (p *servicePackage) DataSources(_ context.Context) []servicepkg.ServicePackageDataSource {
	return nil
}

func (p *servicePackage) ServicePackageName() string {
	return "automation_policy"
}
