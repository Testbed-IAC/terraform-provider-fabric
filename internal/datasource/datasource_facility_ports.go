package datasource

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// FacilityPortsDataSource exposes decoded advertised topology facility ports.
type FacilityPortsDataSource struct {
	client fabricclient.API
}

// NewFacilityPorts returns the typed FABRIC facility ports data source.
func NewFacilityPorts() datasource.DataSource {
	return &FacilityPortsDataSource{}
}

func (d *FacilityPortsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_facility_ports"
}

func (d *FacilityPortsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Decode FABRIC advertised resources into typed facility-port data.",
		MarkdownDescription: "Decode FABRIC advertised resources into typed facility-port data. This data source reads advertised resources at detail level `2` and returns facility ports that can be referenced when designing stitched network services.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Synthetic data source identifier assigned by the provider.", MarkdownDescription: "Synthetic data source identifier assigned by the provider."},
			"name":          schema.StringAttribute{Optional: true, Description: "Optional exact facility port name filter.", MarkdownDescription: "Optional exact facility port name filter."},
			"includes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to include, such as RENC,UKY.", MarkdownDescription: "Comma-separated site codes to include, such as `RENC,UKY`."},
			"excludes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to exclude, such as RENC,UKY.", MarkdownDescription: "Comma-separated site codes to exclude, such as `RENC,UKY`."},
			"force_refresh": schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to bypass cached resource information. Defaults to false.", MarkdownDescription: "Whether to bypass cached resource information. Defaults to `false`."},
			"facility_ports": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Typed FABRIC facility-port resource data assigned by the provider after decoding advertised resources.",
				MarkdownDescription: "Typed FABRIC facility-port resource data assigned by the provider after decoding advertised resources.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":       schema.StringAttribute{Computed: true, Description: "Facility port name assigned by FABRIC advertised resources.", MarkdownDescription: "Facility port name assigned by FABRIC advertised resources."},
					"site":       schema.StringAttribute{Computed: true, Description: "Site that advertises the facility port.", MarkdownDescription: "Site that advertises the facility port."},
					"switch":     schema.StringAttribute{Computed: true, Description: "Switch that hosts the facility port when advertised by FABRIC.", MarkdownDescription: "Switch that hosts the facility port when advertised by FABRIC."},
					"vlan_range": schema.StringAttribute{Computed: true, Description: "Advertised VLAN range for the facility port assigned by FABRIC.", MarkdownDescription: "Advertised VLAN range for the facility port assigned by FABRIC."},
					"bandwidth":  schema.Int64Attribute{Computed: true, Description: "Advertised facility-port bandwidth in Gbps assigned by FABRIC.", MarkdownDescription: "Advertised facility-port bandwidth in Gbps assigned by FABRIC."},
				}},
			},
		},
	}
}

func (d *FacilityPortsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	data, diags := providercfg.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if data == nil {
		return
	}
	d.client = data.Client
}

func (d *FacilityPortsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if err := ctx.Err(); err != nil {
		resp.Diagnostics.AddError("Read FABRIC facility ports cancelled", err.Error())
		return
	}
	var config FacilityPortsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	advertised := decodeAdvertisedResources(ctx, d.client, "facility ports", tfutil.StringValue(config.Includes), tfutil.StringValue(config.Excludes), tfutil.BoolValue(config.ForceRefresh), &resp.Diagnostics)
	if advertised == nil {
		return
	}
	config.ID = types.StringValue("facility_ports")
	config.ForceRefresh = types.BoolValue(tfutil.BoolValue(config.ForceRefresh))
	config.FacilityPorts = facilityPortModels(advertised.FacilityPorts, tfutil.StringValue(config.Name), tfutil.StringValue(config.Includes), tfutil.StringValue(config.Excludes))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func facilityPortModels(ports []catalog.FacilityPort, nameFilter, includes, excludes string) []FacilityPortDataSourceModel {
	out := []FacilityPortDataSourceModel{}
	for _, port := range ports {
		if nameFilter != "" && port.Name != nameFilter {
			continue
		}
		if !siteIncluded(port.Site, includes, excludes) {
			continue
		}
		out = append(out, FacilityPortDataSourceModel{
			Name:      types.StringValue(port.Name),
			Site:      types.StringValue(port.Site),
			Switch:    types.StringValue(port.Switch),
			VLANRange: types.StringValue(port.VLANRange),
			Bandwidth: types.Int64Value(int64(port.Bandwidth)),
		})
	}
	return out
}
