package widget

// The shape kgen crud renders for an entity with create_only: [secret]. secret
// is settable here so the audit has something to decide; note is bound nowhere.

func WidgetResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true},
			"name":   schema.StringAttribute{Required: true},
			"note":   schema.StringAttribute{Optional: true},
			"secret": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true},
		},
	}
}

type WidgetModel struct {
	Id     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Note   types.String `tfsdk:"note"`
	Secret types.String `tfsdk:"secret"`
}

func (r *widgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	conn := r.Meta().Client
	var plan WidgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	input := generated.OptWidgetCreateRequest{
		Value: generated.WidgetCreateRequest{Name: flex.StringValueFromFramework(plan.Name)},
		Set:   true,
	}
	out, err := conn.PostWidget(ctx, input)
	if err != nil {
		return
	}
	created, ok := out.(*generated.WidgetCreatedResponse)
	if !ok {
		return
	}
	id := created.Data.Value.ID.Value
	plan.Id = types.StringValue(id)
	configuredSecret := flex.OptStringToFrameworkNullIfEmpty(created.Data.Value.Secret)
	readOut, _ := conn.GetWidget(ctx, generated.GetWidgetParams{ID: id})
	resp.Diagnostics.Append(flattenWidget(readOut, &plan)...)
	plan.Secret = configuredSecret
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *widgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	conn := r.Meta().Client
	var plan WidgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state WidgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	id := plan.Id.ValueString()
	input := generated.OptWidgetUpdateRequest{
		Value: generated.WidgetUpdateRequest{Name: flex.OptStringFromFramework(plan.Name)},
		Set:   true,
	}
	_, _ = conn.PutWidget(ctx, input, generated.PutWidgetParams{ID: id})
	readOut, _ := conn.GetWidget(ctx, generated.GetWidgetParams{ID: id})
	resp.Diagnostics.Append(flattenWidget(readOut, &plan)...)
	plan.Secret = state.Secret
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func flattenWidget(out generated.GetWidgetRes, model *WidgetModel) diag.Diagnostics {
	v := out.(*generated.WidgetResponse)
	model.Name = flex.OptStringToFramework(v.Data.Value.Name)
	return nil
}
