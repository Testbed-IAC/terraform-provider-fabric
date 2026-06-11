package datasource

import "github.com/hashicorp/terraform-plugin-framework/types"

// SliceDataSourceModel is the state model for the fabric_slice data source: the
// lookup keys and the slice fields returned by the orchestrator.
type SliceDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	SliceID        types.String `tfsdk:"slice_id"`
	Name           types.String `tfsdk:"name"`
	State          types.String `tfsdk:"state"`
	GraphID        types.String `tfsdk:"graph_id"`
	LeaseStartTime types.String `tfsdk:"lease_start_time"`
	LeaseEndTime   types.String `tfsdk:"lease_end_time"`
}

// ResourcesDataSourceModel is the state model for the fabric_resources data
// source: the query filters and the opaque resource model string returned.
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

// SitesDataSourceModel is the state model for the fabric_sites data source: the
// include/exclude filters and the decoded list of sites.
type SitesDataSourceModel struct {
	ID           types.String          `tfsdk:"id"`
	Name         types.String          `tfsdk:"name"`
	Includes     types.String          `tfsdk:"includes"`
	Excludes     types.String          `tfsdk:"excludes"`
	ForceRefresh types.Bool            `tfsdk:"force_refresh"`
	Sites        []SiteDataSourceModel `tfsdk:"sites"`
}

// SiteDataSourceModel is one decoded FABRIC site with its aggregate capacity,
// hosts, and component availability.
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

// HostDataSourceModel is one host within a site with its capacity and component
// availability.
type HostDataSourceModel struct {
	Name       types.String                    `tfsdk:"name"`
	Site       types.String                    `tfsdk:"site"`
	Cores      CapacityAvailabilitySourceModel `tfsdk:"cores"`
	RAM        CapacityAvailabilitySourceModel `tfsdk:"ram"`
	Disk       CapacityAvailabilitySourceModel `tfsdk:"disk"`
	Components []ComponentAvailabilityModel    `tfsdk:"components"`
}

// CapacityAvailabilitySourceModel is a total/allocated/available triple for a
// capacity dimension (cores, RAM, or disk).
type CapacityAvailabilitySourceModel struct {
	Capacity  types.Int64 `tfsdk:"capacity"`
	Allocated types.Int64 `tfsdk:"allocated"`
	Available types.Int64 `tfsdk:"available"`
}

// ComponentAvailabilityModel is the advertised count for one component key, such
// as GPU/RTX6000 or SmartNIC/ConnectX-6.
type ComponentAvailabilityModel struct {
	Name      types.String `tfsdk:"name"`
	Capacity  types.Int64  `tfsdk:"capacity"`
	Allocated types.Int64  `tfsdk:"allocated"`
	Available types.Int64  `tfsdk:"available"`
}

// FacilityPortsDataSourceModel is the state model for the fabric_facility_ports
// data source: the include/exclude filters and the decoded ports.
type FacilityPortsDataSourceModel struct {
	ID            types.String                  `tfsdk:"id"`
	Name          types.String                  `tfsdk:"name"`
	Includes      types.String                  `tfsdk:"includes"`
	Excludes      types.String                  `tfsdk:"excludes"`
	ForceRefresh  types.Bool                    `tfsdk:"force_refresh"`
	FacilityPorts []FacilityPortDataSourceModel `tfsdk:"facility_ports"`
}

// FacilityPortDataSourceModel is one decoded advertised facility port.
type FacilityPortDataSourceModel struct {
	Name      types.String `tfsdk:"name"`
	Site      types.String `tfsdk:"site"`
	Switch    types.String `tfsdk:"switch"`
	VLANRange types.String `tfsdk:"vlan_range"`
	Bandwidth types.Int64  `tfsdk:"bandwidth"`
}

// SliversDataSourceModel is the state model for the fabric_slivers data source:
// the slice ID and the per-sliver state list.
type SliversDataSourceModel struct {
	ID      types.String            `tfsdk:"id"`
	SliceID types.String            `tfsdk:"slice_id"`
	Slivers []SliverDataSourceModel `tfsdk:"slivers"`
}

// SliverDataSourceModel is the runtime state of one sliver in a slice.
type SliverDataSourceModel struct {
	SliverID     types.String `tfsdk:"sliver_id"`
	SliverType   types.String `tfsdk:"sliver_type"`
	State        types.String `tfsdk:"state"`
	PendingState types.String `tfsdk:"pending_state"`
	JoinState    types.String `tfsdk:"join_state"`
	ManagementIP types.String `tfsdk:"management_ip"`
	GraphNodeID  types.String `tfsdk:"graph_node_id"`
}

// MetricsDataSourceModel is the state model for the fabric_metrics data source:
// the excluded-project filter and the JSON results string.
type MetricsDataSourceModel struct {
	ID               types.String   `tfsdk:"id"`
	ExcludedProjects []types.String `tfsdk:"excluded_projects"`
	Results          types.String   `tfsdk:"results"`
}
