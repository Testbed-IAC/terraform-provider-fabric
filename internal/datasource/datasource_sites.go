package datasource

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// SitesDataSource exposes decoded advertised topology site data.
type SitesDataSource struct {
	client fabricclient.API
}

// NewSites returns the typed FABRIC sites data source.
func NewSites() datasource.DataSource {
	return &SitesDataSource{}
}

func (d *SitesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sites"
}

func (d *SitesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Decode FABRIC advertised resources into typed site capacity, host, and component data.",
		MarkdownDescription: "Decode FABRIC advertised resources into typed site capacity, host, and component data. This data source reads advertised resources at detail level `2` and converts the graph into Terraform attributes.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Synthetic data source identifier assigned by the provider.", MarkdownDescription: "Synthetic data source identifier assigned by the provider."},
			"name":          schema.StringAttribute{Optional: true, Description: "Optional exact site name filter.", MarkdownDescription: "Optional exact site name filter."},
			"includes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to include, such as RENC,UKY.", MarkdownDescription: "Comma-separated site codes to include, such as `RENC,UKY`."},
			"excludes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to exclude, such as RENC,UKY.", MarkdownDescription: "Comma-separated site codes to exclude, such as `RENC,UKY`."},
			"force_refresh": schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to bypass cached resource information. Defaults to false.", MarkdownDescription: "Whether to bypass cached resource information. Defaults to `false`."},
			"sites": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Typed FABRIC site resource data assigned by the provider after decoding advertised resources.",
				MarkdownDescription: "Typed FABRIC site resource data assigned by the provider after decoding advertised resources.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":            schema.StringAttribute{Computed: true, Description: "Site name assigned by FABRIC.", MarkdownDescription: "Site name assigned by FABRIC."},
					"cores":           capacityAttribute("CPU core capacity."),
					"ram":             capacityAttribute("RAM capacity."),
					"disk":            capacityAttribute("Disk capacity."),
					"ptp":             schema.BoolAttribute{Computed: true, Description: "Whether the site advertises PTP support. Assigned by FABRIC advertised resources.", MarkdownDescription: "Whether the site advertises PTP support. Assigned by FABRIC advertised resources."},
					"ipv4_management": schema.BoolAttribute{Computed: true, Description: "Whether the site advertises IPv4 management support. Assigned by FABRIC advertised resources.", MarkdownDescription: "Whether the site advertises IPv4 management support. Assigned by FABRIC advertised resources."},
					"hosts": schema.ListNestedAttribute{
						Computed:            true,
						Description:         "Host-level resource data for this site assigned by FABRIC advertised resources.",
						MarkdownDescription: "Host-level resource data for this site assigned by FABRIC advertised resources.",
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"name":       schema.StringAttribute{Computed: true, Description: "Host name assigned by FABRIC advertised resources.", MarkdownDescription: "Host name assigned by FABRIC advertised resources."},
							"site":       schema.StringAttribute{Computed: true, Description: "Site name that contains this host.", MarkdownDescription: "Site name that contains this host."},
							"cores":      capacityAttribute("Host CPU core capacity."),
							"ram":        capacityAttribute("Host RAM capacity."),
							"disk":       capacityAttribute("Host disk capacity."),
							"components": componentsAttribute(),
						}},
					},
					"components": componentsAttribute(),
				}},
			},
		},
	}
}

func (d *SitesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SitesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if err := ctx.Err(); err != nil {
		resp.Diagnostics.AddError("Read FABRIC sites cancelled", err.Error())
		return
	}
	var config SitesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	model, err := d.client.GetResources(ctx, fabricclient.ResourcesQuery{Level: advertisedResourcesLevel, ForceRefresh: tfutil.BoolValue(config.ForceRefresh), Includes: tfutil.StringValue(config.Includes), Excludes: tfutil.StringValue(config.Excludes)})
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC sites failed", err.Error())
		return
	}
	advertised, err := catalog.DecodeAdvertised(model)
	if err != nil {
		resp.Diagnostics.AddError("Decode FABRIC advertised resources failed", err.Error())
		return
	}
	config.ID = types.StringValue("sites")
	config.ForceRefresh = types.BoolValue(tfutil.BoolValue(config.ForceRefresh))
	config.Sites = siteModels(advertised.Sites, tfutil.StringValue(config.Name), tfutil.StringValue(config.Includes), tfutil.StringValue(config.Excludes))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// advertisedResourcesLevel is the FABRIC resource detail level that returns host
// and component data, which the sites and facility-ports decoders require.
const advertisedResourcesLevel int32 = 2

func capacityAttribute(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed:            true,
		Description:         description,
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"capacity":  schema.Int64Attribute{Computed: true, Description: "Total advertised capacity assigned by FABRIC.", MarkdownDescription: "Total advertised capacity assigned by FABRIC."},
			"allocated": schema.Int64Attribute{Computed: true, Description: "Currently allocated capacity assigned by FABRIC.", MarkdownDescription: "Currently allocated capacity assigned by FABRIC."},
			"available": schema.Int64Attribute{Computed: true, Description: "Currently available capacity assigned by FABRIC.", MarkdownDescription: "Currently available capacity assigned by FABRIC."},
		},
	}
}

func componentsAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed:            true,
		Description:         "Component availability data assigned by FABRIC advertised resources.",
		MarkdownDescription: "Component availability data assigned by FABRIC advertised resources.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"name":      schema.StringAttribute{Computed: true, Description: "Component key assigned by FABRIC, such as GPU/A30 or SmartNIC/ConnectX-6.", MarkdownDescription: "Component key assigned by FABRIC, such as `GPU/A30` or `SmartNIC/ConnectX-6`."},
			"capacity":  schema.Int64Attribute{Computed: true, Description: "Total advertised component count assigned by FABRIC.", MarkdownDescription: "Total advertised component count assigned by FABRIC."},
			"allocated": schema.Int64Attribute{Computed: true, Description: "Currently allocated component count assigned by FABRIC.", MarkdownDescription: "Currently allocated component count assigned by FABRIC."},
			"available": schema.Int64Attribute{Computed: true, Description: "Currently available component count assigned by FABRIC.", MarkdownDescription: "Currently available component count assigned by FABRIC."},
		}},
	}
}

func siteModels(sites []catalog.Site, nameFilter, includes, excludes string) []SiteDataSourceModel {
	out := []SiteDataSourceModel{}
	for _, site := range sites {
		if nameFilter != "" && site.Name != nameFilter {
			continue
		}
		if !siteIncluded(site.Name, includes, excludes) {
			continue
		}
		out = append(out, SiteDataSourceModel{
			Name:           types.StringValue(site.Name),
			Cores:          capacityModel(site.Cores),
			RAM:            capacityModel(site.RAM),
			Disk:           capacityModel(site.Disk),
			PTP:            types.BoolValue(site.PTP),
			IPv4Management: types.BoolValue(site.IPv4Management),
			Hosts:          hostModels(site.Hosts),
			Components:     componentModels(site.Components),
		})
	}
	return out
}

func hostModels(hosts []catalog.Host) []HostDataSourceModel {
	out := make([]HostDataSourceModel, 0, len(hosts))
	for _, host := range hosts {
		out = append(out, HostDataSourceModel{
			Name:       types.StringValue(host.Name),
			Site:       types.StringValue(host.Site),
			Cores:      capacityModel(host.Cores),
			RAM:        capacityModel(host.RAM),
			Disk:       capacityModel(host.Disk),
			Components: componentModels(host.Components),
		})
	}
	return out
}

func capacityModel(capacity catalog.CapacityAvailability) CapacityAvailabilitySourceModel {
	return CapacityAvailabilitySourceModel{
		Capacity:  types.Int64Value(int64(capacity.Capacity)),
		Allocated: types.Int64Value(int64(capacity.Allocated)),
		Available: types.Int64Value(int64(capacity.Available)),
	}
}

func componentModels(components map[string]catalog.ComponentAvail) []ComponentAvailabilityModel {
	names := make([]string, 0, len(components))
	for name := range components {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ComponentAvailabilityModel, 0, len(names))
	for _, name := range names {
		component := components[name]
		out = append(out, ComponentAvailabilityModel{
			Name:      types.StringValue(name),
			Capacity:  types.Int64Value(int64(component.Capacity)),
			Allocated: types.Int64Value(int64(component.Allocated)),
			Available: types.Int64Value(int64(component.Available)),
		})
	}
	return out
}

func siteIncluded(siteName, includes, excludes string) bool {
	includeSet := siteFilterSet(includes)
	if len(includeSet) > 0 && !includeSet[siteName] {
		return false
	}
	return !siteFilterSet(excludes)[siteName]
}

func siteFilterSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		site := strings.TrimSpace(part)
		if site != "" {
			out[site] = true
		}
	}
	return out
}
