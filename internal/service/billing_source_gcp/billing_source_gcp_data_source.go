package billing_source_gcp

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
	DSNameBillingSourceGcp = "Billing Source Gcp Data Source"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &billingSourceGcpDataSource{}
	_ datasource.DataSourceWithConfigure = &billingSourceGcpDataSource{}
)

// NewBillingSourceGcpDataSource returns a new instance of the data source.
func NewBillingSourceGcpDataSource() datasource.DataSource {
	return &billingSourceGcpDataSource{}
}

type billingSourceGcpDataSource struct {
	framework.DataSourceWithConfigure
}

func (d *billingSourceGcpDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_source_gcp"
}

func (d *billingSourceGcpDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to access information about Kion Billing Source Gcps.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the Billing Source Gcp.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the Billing Source Gcp.",
				Computed:    true,
			},
		},
	}
}

func (d *billingSourceGcpDataSource) Read(_ context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.AddError("not implemented", "Read for Billing Source Gcp data source is not yet implemented")
}

func flattenBillingSourceGcpDataSource(apiObject any, _ *billingSourceGcpDataSourceModel) diag.Diagnostics {
	// TODO: implement — type-switch the response and map SDK fields to model fields
	_ = apiObject
	return nil
}

type billingSourceGcpDataSourceModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	// TODO: Add fields matching your schema attributes here.
}
