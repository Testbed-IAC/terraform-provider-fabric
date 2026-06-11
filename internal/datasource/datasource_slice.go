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

// SliceDataSource looks up a single FABRIC slice by ID or name.
type SliceDataSource struct {
	client fabricclient.API
}

// NewSlice returns the FABRIC slice data source.
func NewSlice() datasource.DataSource {
	return &SliceDataSource{}
}

func (d *SliceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slice"
}

func (d *SliceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Look up a FABRIC slice by slice_id, id, or name.",
		MarkdownDescription: "Look up a FABRIC slice by `slice_id`, `id`, or `name` using the FABRIC orchestrator. When multiple arguments are set, `slice_id` takes precedence over `id`, and `id` takes precedence over `name`.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Optional: true, Computed: true, Description: "FABRIC slice identifier used for lookup or assigned by the orchestrator after lookup.", MarkdownDescription: "FABRIC slice identifier used for lookup or assigned by the orchestrator after lookup."},
			"slice_id":         schema.StringAttribute{Optional: true, Computed: true, Description: "FABRIC slice identifier used for lookup or assigned by the orchestrator after lookup.", MarkdownDescription: "FABRIC slice identifier used for lookup or assigned by the orchestrator after lookup."},
			"name":             schema.StringAttribute{Optional: true, Computed: true, Description: "Slice name used for lookup or returned by the orchestrator after lookup.", MarkdownDescription: "Slice name used for lookup or returned by the orchestrator after lookup."},
			"state":            schema.StringAttribute{Computed: true, Description: "Current slice state assigned by the orchestrator.", MarkdownDescription: "Current slice state assigned by the orchestrator."},
			"graph_id":         schema.StringAttribute{Computed: true, Description: "Topology graph identifier assigned by FABRIC.", MarkdownDescription: "Topology graph identifier assigned by FABRIC."},
			"lease_start_time": schema.StringAttribute{Computed: true, Description: "Lease start time assigned by FABRIC and normalized by the provider.", MarkdownDescription: "Lease start time assigned by FABRIC and normalized by the provider."},
			"lease_end_time":   schema.StringAttribute{Computed: true, Description: "Lease end time assigned by FABRIC and normalized by the provider.", MarkdownDescription: "Lease end time assigned by FABRIC and normalized by the provider."},
		},
	}
}

func (d *SliceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SliceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SliceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var slice *fabricclient.Slice
	var err error
	if id := tfutil.StringValue(config.SliceID); id != "" {
		slice, err = d.client.GetSlice(ctx, id)
	} else if id := tfutil.StringValue(config.ID); id != "" {
		slice, err = d.client.GetSlice(ctx, id)
	} else if name := tfutil.StringValue(config.Name); name != "" {
		var slices []fabricclient.Slice
		slices, err = d.client.ListSlices(ctx, name, []string{"Nascent", "Configuring", "StableError", "StableOK", "Modifying", "ModifyOK", "ModifyError", "AllocatedOK", "AllocatedError"})
		if err == nil && len(slices) > 0 {
			s := slices[0]
			slice = &s
		}
		if err == nil && slice == nil {
			err = fabricclient.ErrNotFound
		}
	} else {
		resp.Diagnostics.AddError("Missing slice lookup key", "Set slice_id or name.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC slice data source failed", err.Error())
		return
	}
	if slice == nil {
		resp.Diagnostics.AddError("Read FABRIC slice data source failed", "The orchestrator returned no slice.")
		return
	}
	config.ID = types.StringValue(slice.SliceID)
	config.SliceID = types.StringValue(slice.SliceID)
	config.Name = types.StringValue(slice.Name)
	config.State = types.StringValue(slice.State)
	config.GraphID = types.StringValue(slice.GraphID)
	leaseStartTime, err := tfutil.CanonicalFabricTimeString(slice.LeaseStartTime)
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC slice data source failed", "The orchestrator returned an invalid lease_start_time. Original error: "+err.Error())
		return
	}
	leaseEndTime, err := tfutil.CanonicalFabricTimeString(slice.LeaseEndTime)
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC slice data source failed", "The orchestrator returned an invalid lease_end_time. Original error: "+err.Error())
		return
	}
	config.LeaseStartTime = types.StringValue(leaseStartTime)
	config.LeaseEndTime = types.StringValue(leaseEndTime)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
