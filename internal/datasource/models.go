package datasource

import "github.com/hashicorp/terraform-plugin-framework/types"

type SliceDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	SliceID        types.String `tfsdk:"slice_id"`
	Name           types.String `tfsdk:"name"`
	State          types.String `tfsdk:"state"`
	GraphID        types.String `tfsdk:"graph_id"`
	LeaseStartTime types.String `tfsdk:"lease_start_time"`
	LeaseEndTime   types.String `tfsdk:"lease_end_time"`
}

type ResourcesDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Level        types.Int64  `tfsdk:"level"`
	ForceRefresh types.Bool   `tfsdk:"force_refresh"`
	StartDate    types.String `tfsdk:"start_date"`
	EndDate      types.String `tfsdk:"end_date"`
	Includes     types.String `tfsdk:"includes"`
	Excludes     types.String `tfsdk:"excludes"`
	GraphFormat  types.String `tfsdk:"graph_format"`
	Model        types.String `tfsdk:"model"`
}

type SitesDataSourceModel struct {
	ID           types.String          `tfsdk:"id"`
	Name         types.String          `tfsdk:"name"`
	Includes     types.String          `tfsdk:"includes"`
	Excludes     types.String          `tfsdk:"excludes"`
	ForceRefresh types.Bool            `tfsdk:"force_refresh"`
	Sites        []SiteDataSourceModel `tfsdk:"sites"`
}

type SiteDataSourceModel struct {
	Name           types.String                    `tfsdk:"name"`
	Cores          CapacityAvailabilitySourceModel `tfsdk:"cores"`
	RAM            CapacityAvailabilitySourceModel `tfsdk:"ram"`
	Disk           CapacityAvailabilitySourceModel `tfsdk:"disk"`
	PTP            types.Bool                      `tfsdk:"ptp"`
	IPv4Management types.Bool                      `tfsdk:"ipv4_management"`
	Hosts          []HostDataSourceModel           `tfsdk:"hosts"`
	Components     []ComponentAvailabilityModel    `tfsdk:"components"`
}

type HostDataSourceModel struct {
	Name       types.String                    `tfsdk:"name"`
	Site       types.String                    `tfsdk:"site"`
	Cores      CapacityAvailabilitySourceModel `tfsdk:"cores"`
	RAM        CapacityAvailabilitySourceModel `tfsdk:"ram"`
	Disk       CapacityAvailabilitySourceModel `tfsdk:"disk"`
	Components []ComponentAvailabilityModel    `tfsdk:"components"`
}

type CapacityAvailabilitySourceModel struct {
	Capacity  types.Int64 `tfsdk:"capacity"`
	Allocated types.Int64 `tfsdk:"allocated"`
	Available types.Int64 `tfsdk:"available"`
}

type ComponentAvailabilityModel struct {
	Name      types.String `tfsdk:"name"`
	Capacity  types.Int64  `tfsdk:"capacity"`
	Allocated types.Int64  `tfsdk:"allocated"`
	Available types.Int64  `tfsdk:"available"`
}

type FacilityPortsDataSourceModel struct {
	ID            types.String                  `tfsdk:"id"`
	Name          types.String                  `tfsdk:"name"`
	Includes      types.String                  `tfsdk:"includes"`
	Excludes      types.String                  `tfsdk:"excludes"`
	ForceRefresh  types.Bool                    `tfsdk:"force_refresh"`
	FacilityPorts []FacilityPortDataSourceModel `tfsdk:"facility_ports"`
}

type FacilityPortDataSourceModel struct {
	Name      types.String `tfsdk:"name"`
	Site      types.String `tfsdk:"site"`
	Switch    types.String `tfsdk:"switch"`
	VLANRange types.String `tfsdk:"vlan_range"`
	Bandwidth types.Int64  `tfsdk:"bandwidth"`
}

type SliversDataSourceModel struct {
	ID      types.String            `tfsdk:"id"`
	SliceID types.String            `tfsdk:"slice_id"`
	Slivers []SliverDataSourceModel `tfsdk:"slivers"`
}

type SliverDataSourceModel struct {
	SliverID     types.String `tfsdk:"sliver_id"`
	SliverType   types.String `tfsdk:"sliver_type"`
	State        types.String `tfsdk:"state"`
	PendingState types.String `tfsdk:"pending_state"`
	JoinState    types.String `tfsdk:"join_state"`
	ManagementIP types.String `tfsdk:"management_ip"`
	GraphNodeID  types.String `tfsdk:"graph_node_id"`
}

type MetricsDataSourceModel struct {
	ID               types.String   `tfsdk:"id"`
	ExcludedProjects []types.String `tfsdk:"excluded_projects"`
	Results          types.String   `tfsdk:"results"`
}
