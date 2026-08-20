package cft

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
		{Factory: NewCftResource},
		{Factory: NewAwsCftResource}, // backwards-compat alias: kion_aws_cloudformation_template
	}
}

func (p *servicePackage) DataSources(_ context.Context) []servicepkg.ServicePackageDataSource {
	return []servicepkg.ServicePackageDataSource{
		{Factory: NewCftDataSource},
		{Factory: NewAwsCftDataSource}, // backwards-compat alias: kion_aws_cloudformation_template
	}
}

func (p *servicePackage) ServicePackageName() string {
	return "cft"
}
