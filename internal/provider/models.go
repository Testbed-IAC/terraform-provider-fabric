package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

type FabricProviderData struct {
	Client          fabricclient.FabricClient
	TokenSource     fabricclient.TokenSource
	ResourcesSource resourcesSummarySource
}

type resourcesSummarySource interface {
	GetResourcesSummary(ctx context.Context, opts catalog.ResourcesOptions) (*catalog.ResourcesSummary, error)
}

type FabricProviderModel struct {
	Token           types.String `tfsdk:"token"`
	TokenFile       types.String `tfsdk:"token_file"`
	OrchestratorURL types.String `tfsdk:"orchestrator_url"`
	CredmgrURL      types.String `tfsdk:"credmgr_url"`
}

type SliceResourceModel struct {
	ID             types.String   `tfsdk:"id"`
	SliceID        types.String   `tfsdk:"slice_id"`
	GraphID        types.String   `tfsdk:"graph_id"`
	Name           types.String   `tfsdk:"name"`
	SSHKey         types.String   `tfsdk:"ssh_key"`
	SSHKeys        types.List     `tfsdk:"ssh_keys"`
	SSHKeyVersion  types.Int64    `tfsdk:"ssh_key_version"`
	LifetimeHours  types.Int64    `tfsdk:"lifetime_hours"`
	LeaseStartTime types.String   `tfsdk:"lease_start_time"`
	LeaseEndTime   types.String   `tfsdk:"lease_end_time"`
	State          types.String   `tfsdk:"state"`
	Nodes          []NodeModel    `tfsdk:"node"`
	Networks       []NetworkModel `tfsdk:"network"`
	NodeOutputs    types.Map      `tfsdk:"nodes"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
}

type NodeModel struct {
	Name         types.String     `tfsdk:"name"`
	Site         types.String     `tfsdk:"site"`
	Host         types.String     `tfsdk:"host"`
	InstanceType types.String     `tfsdk:"instance_type"`
	ImageRef     types.String     `tfsdk:"image_ref"`
	ImageType    types.String     `tfsdk:"image_type"`
	Cores        types.Int64      `tfsdk:"cores"`
	RAM          types.Int64      `tfsdk:"ram"`
	Disk         types.Int64      `tfsdk:"disk"`
	Labels       labelsModel      `tfsdk:"labels"`
	Components   []ComponentModel `tfsdk:"component"`
}

type ComponentModel struct {
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Model      types.String `tfsdk:"model"`
	FABlibName types.String `tfsdk:"fablib_name"`
	Labels     labelsModel  `tfsdk:"labels"`
}

type NetworkModel struct {
	Name            types.String     `tfsdk:"name"`
	Type            types.String     `tfsdk:"type"`
	Bandwidth       types.Int64      `tfsdk:"bandwidth"`
	Interfaces      []InterfaceModel `tfsdk:"interface"`
	MirrorFrom      types.String     `tfsdk:"mirror_from"`
	MirrorDirection types.String     `tfsdk:"mirror_direction"`
	Labels          labelsModel      `tfsdk:"labels"`
}

type InterfaceModel struct {
	Node      types.String `tfsdk:"node"`
	Component types.String `tfsdk:"component"`
	Port      types.Int64  `tfsdk:"port"`
	Name      types.String `tfsdk:"name"`
	Labels    labelsModel  `tfsdk:"labels"`
}

type NodeOutputModel struct {
	ManagementIP     types.String `tfsdk:"management_ip"`
	SliverID         types.String `tfsdk:"sliver_id"`
	State            types.String `tfsdk:"state"`
	GraphNodeID      types.String `tfsdk:"graph_node_id"`
	ReservationState types.String `tfsdk:"reservation_state"`
	ErrorMessage     types.String `tfsdk:"error_message"`
}

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

type POAResourceModel struct {
	ID         types.String      `tfsdk:"id"`
	POAID      types.String      `tfsdk:"poa_id"`
	SliverID   types.String      `tfsdk:"sliver_id"`
	Operation  types.String      `tfsdk:"operation"`
	VCPUCPUMap []POAVCPUCPUModel `tfsdk:"vcpu_cpu_map"`
	NodeSet    []types.String    `tfsdk:"node_set"`
	BDF        []types.String    `tfsdk:"bdf"`
	Keys       []POAKeyModel     `tfsdk:"keys"`
	Triggers   types.Map         `tfsdk:"triggers"`
	State      types.String      `tfsdk:"state"`
	Error      types.String      `tfsdk:"error"`
	Info       types.String      `tfsdk:"info"`
	Timeouts   timeouts.Value    `tfsdk:"timeouts"`
}

type POAVCPUCPUModel struct {
	VCPU types.String `tfsdk:"vcpu"`
	CPU  types.String `tfsdk:"cpu"`
}

type POAKeyModel struct {
	Key     types.String `tfsdk:"key"`
	Comment types.String `tfsdk:"comment"`
}

type MetricsDataSourceModel struct {
	ID               types.String   `tfsdk:"id"`
	ExcludedProjects []types.String `tfsdk:"excluded_projects"`
	Results          types.String   `tfsdk:"results"`
}
