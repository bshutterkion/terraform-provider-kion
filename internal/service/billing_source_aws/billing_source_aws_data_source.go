package billing_source_aws

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
	DSNameBillingSourceAws = "Billing Source Aws Data Source"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &billingSourceAwsDataSource{}
	_ datasource.DataSourceWithConfigure = &billingSourceAwsDataSource{}
)

// NewBillingSourceAwsDataSource returns a new instance of the data source.
func NewBillingSourceAwsDataSource() datasource.DataSource {
	return &billingSourceAwsDataSource{}
}

type billingSourceAwsDataSource struct {
	framework.DataSourceWithConfigure
}

func (d *billingSourceAwsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_source_aws"
}

func (d *billingSourceAwsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to access information about Kion Billing Source Awss.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the Billing Source Aws.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the Billing Source Aws.",
				Computed:    true,
			},
		},
	}
}

func (d *billingSourceAwsDataSource) Read(_ context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.AddError("not implemented", "Read for Billing Source Aws data source is not yet implemented")
}

func flattenBillingSourceAwsDataSource(apiObject any, _ *billingSourceAwsDataSourceModel) diag.Diagnostics {
	// TODO: implement, type-switch the response and map SDK fields to model fields
	_ = apiObject
	return nil
}

type billingSourceAwsDataSourceModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	// TODO: Add fields matching your schema attributes here.
}
