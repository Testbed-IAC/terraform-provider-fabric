package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

type providerData struct {
	client      fabricclient.FabricClient
	projectTags map[string]bool
	// jwtProjectTags holds only the tags discovered from the FABRIC token.
	// It is authoritative for what the orchestrator actually grants and is
	// used in user-facing error messages. projectTags above may include
	// additional values the user listed manually in the provider block.
	jwtProjectTags map[string]bool
	projectID      string
}

type FabricProviderModel struct {
	Token           types.String `tfsdk:"token"`
	OrchestratorURL types.String `tfsdk:"orchestrator_url"`
	ProjectID       types.String `tfsdk:"project_id"`
	ProjectTags     types.List   `tfsdk:"project_tags"`
}

type SliceResourceModel struct {
	ID             types.String   `tfsdk:"id"`
	SliceID        types.String   `tfsdk:"slice_id"`
	GraphID        types.String   `tfsdk:"graph_id"`
	Name           types.String   `tfsdk:"name"`
	SSHKey         types.String   `tfsdk:"ssh_key"`
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
	InstanceType types.String     `tfsdk:"instance_type"`
	ImageRef     types.String     `tfsdk:"image_ref"`
	ImageType    types.String     `tfsdk:"image_type"`
	Cores        types.Int64      `tfsdk:"cores"`
	RAM          types.Int64      `tfsdk:"ram"`
	Disk         types.Int64      `tfsdk:"disk"`
	Components   []ComponentModel `tfsdk:"component"`
}

type ComponentModel struct {
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Model      types.String `tfsdk:"model"`
	FABlibName types.String `tfsdk:"fablib_name"`
}

type NetworkModel struct {
	Name            types.String     `tfsdk:"name"`
	Type            types.String     `tfsdk:"type"`
	Bandwidth       types.Int64      `tfsdk:"bandwidth"`
	Interfaces      []InterfaceModel `tfsdk:"interface"`
	MirrorFrom      types.String     `tfsdk:"mirror_from"`
	MirrorDirection types.String     `tfsdk:"mirror_direction"`
}

type InterfaceModel struct {
	Node      types.String `tfsdk:"node"`
	Component types.String `tfsdk:"component"`
	Port      types.Int64  `tfsdk:"port"`
	Name      types.String `tfsdk:"name"`
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
	Model        types.String `tfsdk:"model"`
}
