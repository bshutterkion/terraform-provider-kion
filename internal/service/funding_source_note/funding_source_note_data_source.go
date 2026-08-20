package funding_source_note

// **PLEASE DELETE THIS AND ALL TIP COMMENTS BEFORE SUBMITTING A PR FOR REVIEW!**
//
// TIP: ==== INTRODUCTION ====
// Thank you for trying the kgen tool!
//
// You have opted to include these helpful comments. They all include "TIP:"
// to help you find and remove them when you're done with them.
//
// While some aspects of this file are customized to your input, the
// scaffold tool does *not* look at the Kion API and ensure it has correct
// function, structure, and variable names. It makes guesses based on
// commonalities. You will need to make significant adjustments.
//
// In other words, as generated, this is a rough outline of the work you will
// need to do. If something doesn't make sense for your situation, get rid of
// it.

import (
	// TIP: ==== IMPORTS ====
	// This is a common set of imports but not customized to your code since
	// your code hasn't been written yet. Make sure you, your IDE, or
	// goimports -w <file> fixes these imports.
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-kion/internal/framework"
)

// TIP: ==== FILE STRUCTURE ====
// All data sources should follow this basic outline.
//
// 1. Package declaration
// 2. Imports
// 3. Main data source struct with schema method
// 4. Read method
// 5. Other functions (flatteners, etc.)

// Data source name constant.
const (
	DSNameFundingSourceNote = "Funding Source Note Data Source"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &fundingSourceNoteDataSource{}
	_ datasource.DataSourceWithConfigure = &fundingSourceNoteDataSource{}
)

// NewFundingSourceNoteDataSource returns a new instance of the data source.
func NewFundingSourceNoteDataSource() datasource.DataSource {
	return &fundingSourceNoteDataSource{}
}

type fundingSourceNoteDataSource struct {
	framework.DataSourceWithConfigure
}

func (d *fundingSourceNoteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_funding_source_note"
}

// TIP: ==== SCHEMA ====
// Data sources typically have filter criteria as arguments
// and the results as computed attributes.
func (d *fundingSourceNoteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to access information about Kion Funding Source Notes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the Funding Source Note.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the Funding Source Note.",
				Computed:    true,
			},
		},
	}
}

func (d *fundingSourceNoteDataSource) Read(_ context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	// TIP: ==== DATA SOURCE READ ====
	// 1. conn := d.Meta().Client
	// 2. Get config: req.Config.Get(ctx, &data)
	// 3. Call SDK: out, err := conn.GetXxx(ctx, generated.GetXxxParams{ID: data.ID.ValueInt64()})
	// 4. If errs.IsNotFound(out): add error diagnostic
	// 5. Flatten response into model
	// 6. Set state: resp.State.Set(ctx, &data)
	resp.Diagnostics.AddError("not implemented", "Read for Funding Source Note data source is not yet implemented")
}

// TIP: ==== FLATTEN ====
// flattenFundingSourceNoteDataSource converts an SDK response into the Terraform model.
// See the resource template for examples of type-switching.
func flattenFundingSourceNoteDataSource(apiObject any, _ *fundingSourceNoteDataSourceModel) diag.Diagnostics {
	// TODO: implement — type-switch the response and map SDK fields to model fields
	_ = apiObject
	return nil
}

// TIP: ==== DATA STRUCTURES ====
// These structs should match the schema definition exactly.
type fundingSourceNoteDataSourceModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	// TODO: Add fields matching your schema attributes here.
}
