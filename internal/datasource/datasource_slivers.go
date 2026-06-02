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

// SliversDataSource exposes per-sliver state for a FABRIC slice.
type SliversDataSource struct {
	client fabricclient.API
}

// NewSlivers returns the FABRIC slivers data source.
func NewSlivers() datasource.DataSource {
	return &SliversDataSource{}
}

func (d *SliversDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slivers"
}

func (d *SliversDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read per-sliver state for a FABRIC slice.",
		MarkdownDescription: "Read per-sliver state for a FABRIC slice from the orchestrator. Use this data source to inspect node, network, and component sliver identifiers and runtime state after a slice is provisioned.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, Description: "Synthetic data source identifier assigned by the provider.", MarkdownDescription: "Synthetic data source identifier assigned by the provider."},
			"slice_id": schema.StringAttribute{Required: true, Description: "FABRIC slice identifier whose slivers should be read.", MarkdownDescription: "FABRIC slice identifier whose slivers should be read."},
			"slivers": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Per-sliver state assigned by the orchestrator after provisioning or modification.",
				MarkdownDescription: "Per-sliver state assigned by the orchestrator after provisioning or modification.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"sliver_id":     schema.StringAttribute{Computed: true, Description: "FABRIC sliver identifier assigned by the orchestrator.", MarkdownDescription: "FABRIC sliver identifier assigned by the orchestrator."},
					"sliver_type":   schema.StringAttribute{Computed: true, Description: "FABRIC sliver type assigned by the orchestrator.", MarkdownDescription: "FABRIC sliver type assigned by the orchestrator."},
					"state":         schema.StringAttribute{Computed: true, Description: "Current sliver state assigned by the orchestrator.", MarkdownDescription: "Current sliver state assigned by the orchestrator."},
					"pending_state": schema.StringAttribute{Computed: true, Description: "Pending sliver state when advertised by the orchestrator.", MarkdownDescription: "Pending sliver state when advertised by the orchestrator."},
					"join_state":    schema.StringAttribute{Computed: true, Description: "Join state when advertised by the orchestrator.", MarkdownDescription: "Join state when advertised by the orchestrator."},
					"management_ip": schema.StringAttribute{Computed: true, Description: "Management IP address when advertised by the sliver payload.", MarkdownDescription: "Management IP address when advertised by the sliver payload."},
					"graph_node_id": schema.StringAttribute{Computed: true, Description: "Graph node identifier associated with the sliver.", MarkdownDescription: "Graph node identifier associated with the sliver."},
				}},
			},
		},
	}
}

func (d *SliversDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providercfg.Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return
	}
	d.client = data.Client
}

func (d *SliversDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if err := ctx.Err(); err != nil {
		resp.Diagnostics.AddError("Read FABRIC slivers cancelled", err.Error())
		return
	}
	var config SliversDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sliceID := tfutil.StringValue(config.SliceID)
	slivers, err := d.client.GetSlivers(ctx, sliceID)
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC slivers failed", err.Error())
		return
	}
	config.ID = types.StringValue(sliceID)
	config.Slivers = sliverModels(slivers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func sliverModels(slivers []fabricclient.Sliver) []SliverDataSourceModel {
	out := make([]SliverDataSourceModel, 0, len(slivers))
	for _, sliver := range slivers {
		out = append(out, SliverDataSourceModel{
			SliverID:     types.StringValue(sliver.SliverID),
			SliverType:   types.StringValue(sliver.SliverType),
			State:        types.StringValue(sliver.State),
			PendingState: types.StringValue(sliver.PendingState),
			JoinState:    types.StringValue(sliver.JoinState),
			ManagementIP: types.StringValue(sliver.ManagementIP),
			GraphNodeID:  types.StringValue(sliver.GraphNodeID),
		})
	}
	return out
}
