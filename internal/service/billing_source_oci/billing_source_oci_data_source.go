package billing_source_oci

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-kion/internal/framework"
)

// Data source name constant.
const (
	DSNameBillingSourceOci = "Billing Source Oci Data Source"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &billingSourceOciDataSource{}
	_ datasource.DataSourceWithConfigure = &billingSourceOciDataSource{}
)

// NewBillingSourceOciDataSource returns a new instance of the data source.
func NewBillingSourceOciDataSource() datasource.DataSource {
	return &billingSourceOciDataSource{}
}

type billingSourceOciDataSource struct {
	framework.DataSourceWithConfigure
}

func (d *billingSourceOciDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_source_oci"
}

func (d *billingSourceOciDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to access information about Kion Billing Source Ocis.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the Billing Source Oci.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the Billing Source Oci.",
				Computed:    true,
			},
		},
	}
}

func (d *billingSourceOciDataSource) Read(_ context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.AddError("not implemented", "Read for Billing Source Oci data source is not yet implemented")
}

func flattenBillingSourceOciDataSource(apiObject any, _ *billingSourceOciDataSourceModel) diag.Diagnostics {
	// TODO: implement — type-switch the response and map SDK fields to model fields
	_ = apiObject
	return nil
}

type billingSourceOciDataSourceModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	// TODO: Add fields matching your schema attributes here.
}
