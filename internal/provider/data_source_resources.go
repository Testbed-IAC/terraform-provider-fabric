package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

type ResourcesDataSource struct {
	client fabricclient.FabricClient
}

func NewResourcesDataSource() datasource.DataSource {
	return &ResourcesDataSource{}
}

func (d *ResourcesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resources"
}

func (d *ResourcesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetch the FABRIC available resources model.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"level":         schema.Int64Attribute{Optional: true, Computed: true},
			"force_refresh": schema.BoolAttribute{Optional: true, Computed: true},
			"model":         schema.StringAttribute{Computed: true},
		},
	}
}

func (d *ResourcesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return
	}
	d.client = data.client
}

func (d *ResourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ResourcesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	level := int64Value(config.Level)
	if level == 0 {
		level = 1
	}
	model, err := d.client.GetResources(ctx, int32(level), boolValue(config.ForceRefresh))
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC resources failed", err.Error())
		return
	}
	config.ID = types.StringValue("resources")
	config.Level = types.Int64Value(level)
	config.ForceRefresh = types.BoolValue(boolValue(config.ForceRefresh))
	config.Model = types.StringValue(model)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
