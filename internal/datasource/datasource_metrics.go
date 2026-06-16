package datasource

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// MetricsDataSource exposes FABRIC metrics overview JSON.
type MetricsDataSource struct {
	client fabricclient.API
}

// NewMetrics returns the FABRIC metrics data source.
func NewMetrics() datasource.DataSource {
	return &MetricsDataSource{}
}

func (d *MetricsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metrics"
}

func (d *MetricsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read the FABRIC metrics overview as JSON.",
		MarkdownDescription: "Read the FABRIC metrics overview as JSON from the orchestrator metrics endpoint.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Synthetic data source identifier assigned by the provider.",
				MarkdownDescription: "Synthetic data source identifier assigned by the provider.",
			},
			"excluded_projects": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Project identifiers to exclude from the metrics overview. Defaults to no excluded projects.",
				MarkdownDescription: "Project identifiers to exclude from the metrics overview. Defaults to no excluded projects.",
			},
			"results": schema.StringAttribute{
				Computed:            true,
				Description:         "Metrics overview results encoded as JSON and assigned by the provider after lookup.",
				MarkdownDescription: "Metrics overview results encoded as JSON and assigned by the provider after lookup.",
			},
		},
	}
}

func (d *MetricsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	data, diags := providercfg.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if data == nil {
		return
	}
	d.client = data.Client
}

func (d *MetricsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if err := ctx.Err(); err != nil {
		resp.Diagnostics.AddError("Read FABRIC metrics cancelled", err.Error())
		return
	}
	var config MetricsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	results, err := d.client.GetMetricsOverview(ctx, fabricclient.MetricsQuery{ExcludedProjects: tfutil.StringValues(config.ExcludedProjects)})
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC metrics failed", err.Error())
		return
	}
	config.ID = types.StringValue("metrics")
	config.Results = types.StringValue(results)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
