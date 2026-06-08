package slice

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type SliceResourceModel struct {
	ID             types.String        `tfsdk:"id"`
	SliceID        types.String        `tfsdk:"slice_id"`
	GraphID        types.String        `tfsdk:"graph_id"`
	Name           types.String        `tfsdk:"name"`
	SSHKey         types.String        `tfsdk:"ssh_key"`
	SSHKeys        types.List          `tfsdk:"ssh_keys"`
	SSHKeyVersion  types.Int64         `tfsdk:"ssh_key_version"`
	LifetimeHours  types.Int64         `tfsdk:"lifetime_hours"`
	LeaseStartTime types.String        `tfsdk:"lease_start_time"`
	LeaseEndTime   types.String        `tfsdk:"lease_end_time"`
	State          types.String        `tfsdk:"state"`
	Nodes          []NodeModel         `tfsdk:"node"`
	Networks       []NetworkModel      `tfsdk:"network"`
	Facilities     []FacilityPortModel `tfsdk:"facility_port"`
	Switches       []SwitchModel       `tfsdk:"switch"`
	NodeOutputs    types.Map           `tfsdk:"nodes"`
	Timeouts       timeouts.Value      `tfsdk:"timeouts"`
}

type NodeModel struct {
	Name            types.String          `tfsdk:"name"`
	Site            types.String          `tfsdk:"site"`
	Host            types.String          `tfsdk:"host"`
	InstanceType    types.String          `tfsdk:"instance_type"`
	ImageRef        types.String          `tfsdk:"image_ref"`
	ImageType       types.String          `tfsdk:"image_type"`
	Cores           types.Int64           `tfsdk:"cores"`
	RAM             types.Int64           `tfsdk:"ram"`
	Disk            types.Int64           `tfsdk:"disk"`
	BootScript      types.String          `tfsdk:"boot_script"`
	PostBootExecute types.List            `tfsdk:"post_boot_execute"`
	PostUpdate      types.List            `tfsdk:"post_update"`
	Labels          *labelsModel          `tfsdk:"labels"`
	Components      []ComponentModel      `tfsdk:"component"`
	Storage         []StorageModel        `tfsdk:"storage"`
	Routes          []RouteModel          `tfsdk:"route"`
	PostBootUploads []PostBootUploadModel `tfsdk:"post_boot_upload"`
}

type ComponentModel struct {
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Model      types.String `tfsdk:"model"`
	FABlibName types.String `tfsdk:"fablib_name"`
	Labels     *labelsModel `tfsdk:"labels"`
}

type StorageModel struct {
	Name      types.String `tfsdk:"name"`
	Model     types.String `tfsdk:"model"`
	AutoMount types.Bool   `tfsdk:"auto_mount"`
}

type RouteModel struct {
	Subnet  types.String `tfsdk:"subnet"`
	NextHop types.String `tfsdk:"next_hop"`
}

type PostBootUploadModel struct {
	LocalPath  types.String `tfsdk:"local_path"`
	RemotePath types.String `tfsdk:"remote_path"`
}

type NetworkModel struct {
	Name            types.String     `tfsdk:"name"`
	Type            types.String     `tfsdk:"type"`
	Bandwidth       types.Int64      `tfsdk:"bandwidth"`
	Site            types.String     `tfsdk:"site"`
	Technology      types.String     `tfsdk:"technology"`
	Subnet          types.String     `tfsdk:"subnet"`
	Interfaces      []InterfaceModel `tfsdk:"interface"`
	Gateway         *GatewayModel    `tfsdk:"gateway"`
	MirrorFrom      types.String     `tfsdk:"mirror_from"`
	MirrorDirection types.String     `tfsdk:"mirror_direction"`
	Labels          *labelsModel     `tfsdk:"labels"`
}

type GatewayModel struct {
	IPv4       types.String `tfsdk:"ipv4"`
	IPv4Subnet types.String `tfsdk:"ipv4_subnet"`
	IPv6       types.String `tfsdk:"ipv6"`
	IPv6Subnet types.String `tfsdk:"ipv6_subnet"`
	MAC        types.String `tfsdk:"mac"`
}

type InterfaceModel struct {
	Node          types.String        `tfsdk:"node"`
	Component     types.String        `tfsdk:"component"`
	Facility      types.String        `tfsdk:"facility"`
	Port          types.Int64         `tfsdk:"port"`
	Name          types.String        `tfsdk:"name"`
	Labels        *labelsModel        `tfsdk:"labels"`
	SubInterfaces []SubInterfaceModel `tfsdk:"sub_interface"`
}

type SubInterfaceModel struct {
	Name      types.String `tfsdk:"name"`
	VLAN      types.String `tfsdk:"vlan"`
	Bandwidth types.Int64  `tfsdk:"bandwidth"`
	Labels    *labelsModel `tfsdk:"labels"`
}

type FacilityPortModel struct {
	Name       types.String                 `tfsdk:"name"`
	Site       types.String                 `tfsdk:"site"`
	VLAN       types.String                 `tfsdk:"vlan"`
	Bandwidth  types.Int64                  `tfsdk:"bandwidth"`
	MTU        types.Int64                  `tfsdk:"mtu"`
	Labels     *labelsModel                 `tfsdk:"labels"`
	Interfaces []FacilityPortInterfaceModel `tfsdk:"interface"`
}

type FacilityPortInterfaceModel struct {
	Name   types.String `tfsdk:"name"`
	VLAN   types.String `tfsdk:"vlan"`
	Labels *labelsModel `tfsdk:"labels"`
}

type SwitchModel struct {
	Name       types.String `tfsdk:"name"`
	Site       types.String `tfsdk:"site"`
	NPorts     types.Int64  `tfsdk:"nports"`
	PortLabels *labelsModel `tfsdk:"port_labels"`
}

type NodeOutputModel struct {
	ManagementIP     types.String `tfsdk:"management_ip"`
	SliverID         types.String `tfsdk:"sliver_id"`
	State            types.String `tfsdk:"state"`
	GraphNodeID      types.String `tfsdk:"graph_node_id"`
	ReservationState types.String `tfsdk:"reservation_state"`
	ErrorMessage     types.String `tfsdk:"error_message"`
}
