package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

// FacilityPortsDataSource exposes decoded advertised topology facility ports.
type FacilityPortsDataSource struct {
	client fabricclient.API
}

// NewFacilityPortsDataSource returns the typed FABRIC facility ports data source.
func NewFacilityPortsDataSource() datasource.DataSource {
	return &FacilityPortsDataSource{}
}

func (d *FacilityPortsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_facility_ports"
}

func (d *FacilityPortsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Decode FABRIC advertised resources into typed facility-port data.",
		MarkdownDescription: "Decode FABRIC advertised resources into typed facility-port data.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Synthetic data source identifier.", MarkdownDescription: "Synthetic data source identifier."},
			"name":          schema.StringAttribute{Optional: true, Description: "Optional exact facility port name filter.", MarkdownDescription: "Optional exact facility port name filter."},
			"includes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to include.", MarkdownDescription: "Comma-separated site codes to include."},
			"excludes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to exclude.", MarkdownDescription: "Comma-separated site codes to exclude."},
			"force_refresh": schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to bypass cached resource information.", MarkdownDescription: "Whether to bypass cached resource information."},
			"facility_ports": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Typed FABRIC facility-port resource data.",
				MarkdownDescription: "Typed FABRIC facility-port resource data.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":       schema.StringAttribute{Computed: true, Description: "Facility port name.", MarkdownDescription: "Facility port name."},
					"site":       schema.StringAttribute{Computed: true, Description: "Site that advertises the facility port.", MarkdownDescription: "Site that advertises the facility port."},
					"switch":     schema.StringAttribute{Computed: true, Description: "Switch that hosts the facility port when advertised.", MarkdownDescription: "Switch that hosts the facility port when advertised."},
					"vlan_range": schema.StringAttribute{Computed: true, Description: "Advertised VLAN range for the facility port.", MarkdownDescription: "Advertised VLAN range for the facility port."},
					"bandwidth":  schema.Int64Attribute{Computed: true, Description: "Advertised facility-port bandwidth in Gbps.", MarkdownDescription: "Advertised facility-port bandwidth in Gbps."},
				}},
			},
		},
	}
}

func (d *FacilityPortsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*FabricProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
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
	model, err := d.client.GetResources(ctx, fabricclient.ResourcesQuery{Level: advertisedResourcesLevel, ForceRefresh: boolValue(config.ForceRefresh), Includes: stringValue(config.Includes), Excludes: stringValue(config.Excludes)})
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC facility ports failed", err.Error())
		return
	}
	advertised, err := catalog.DecodeAdvertised(model)
	if err != nil {
		resp.Diagnostics.AddError("Decode FABRIC advertised resources failed", err.Error())
		return
	}
	config.ID = types.StringValue("facility_ports")
	config.ForceRefresh = types.BoolValue(boolValue(config.ForceRefresh))
	config.FacilityPorts = facilityPortModels(advertised.FacilityPorts, stringValue(config.Name), stringValue(config.Includes), stringValue(config.Excludes))
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
