package slice

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// sliceResourceSchema returns the Terraform schema for the fabric_slice resource.
func sliceResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "A FABRIC slice, including compute nodes, components, facility ports, switches, and network services.",
		MarkdownDescription: "A FABRIC slice, including compute nodes, components, facility ports, switches, and network services. The resource builds a FIM topology and submits it to the [FABRIC orchestrator](https://fabric-testbed.net/). Slice creation and modification are asynchronous; Terraform waits for the orchestrator to reach a terminal state before saving computed runtime fields such as sliver IDs and management IPs.\n\n" +
			"~> **Note:** Changing `name`, `ssh_key`, `ssh_keys`, or `ssh_key_version` forces the slice to be destroyed and recreated.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "The FABRIC slice identifier assigned by the orchestrator after the slice is created.",
				MarkdownDescription: "The FABRIC slice identifier assigned by the orchestrator after the slice is created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slice_id": schema.StringAttribute{
				Computed:            true,
				Description:         "The FABRIC slice identifier assigned by the orchestrator after the slice is created.",
				MarkdownDescription: "The FABRIC slice identifier assigned by the orchestrator after the slice is created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"graph_id": schema.StringAttribute{
				Computed:            true,
				Description:         "The topology graph identifier assigned by FABRIC after the slice topology is accepted.",
				MarkdownDescription: "The topology graph identifier assigned by FABRIC after the slice topology is accepted.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "The slice name shown in FABRIC. Must be unique enough for your project workflow. Changing this value forces the slice to be destroyed and recreated.",
				MarkdownDescription: "The slice name shown in FABRIC. Must be unique enough for your project workflow.\n\n~> **Note:** Changing this value forces the slice to be destroyed and recreated.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ssh_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				DeprecationMessage:  "Use ssh_keys instead. ssh_key remains as a single-key compatibility alias for this release.",
				Description:         "Deprecated single SSH public key compatibility alias. Use ssh_keys for one or more keys. Changing this value forces the slice to be destroyed and recreated. This value is masked in plan output and state.",
				MarkdownDescription: "Deprecated single SSH public key compatibility alias. Use `ssh_keys` for one or more keys. This value is masked in plan output and state.\n\n~> **Note:** Changing this value forces the slice to be destroyed and recreated.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ssh_keys": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Sensitive:           true,
				Description:         "SSH public keys to install on slice nodes. Configure exactly one of ssh_keys or the deprecated ssh_key alias. Changing this value forces the slice to be destroyed and recreated. These values are masked in plan output and state.",
				MarkdownDescription: "SSH public keys to install on slice nodes. Configure exactly one of `ssh_keys` or the deprecated `ssh_key` alias. These values are masked in plan output and state.\n\n~> **Note:** Changing this value forces the slice to be destroyed and recreated.",
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"ssh_key_version": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
				Description:         "User-controlled version marker for rotating SSH keys. Defaults to 0. Changing this value forces the slice to be destroyed and recreated.",
				MarkdownDescription: "User-controlled version marker for rotating SSH keys. Defaults to `0`.\n\n~> **Note:** Changing this value forces the slice to be destroyed and recreated.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"lifetime_hours": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(24),
				Description:         "Requested slice lease duration in hours. Must be at least 1. Defaults to 24.",
				MarkdownDescription: "Requested slice lease duration in hours. Must be at least `1`. Defaults to `24`.",
				Validators:          []validator.Int64{int64validator.AtLeast(1)},
			},
			"lease_start_time": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Requested lease start time in FABRIC time or RFC3339 format. When omitted, the orchestrator assigns the lease start time after provisioning.",
				MarkdownDescription: "Requested lease start time in FABRIC time or RFC3339 format. When omitted, the orchestrator assigns the lease start time after provisioning.",
				Validators:          []validator.String{tfutil.FabricTimeValidator{}},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"lease_end_time": schema.StringAttribute{
				Computed:            true,
				Description:         "Lease end time assigned by the orchestrator after provisioning or renewal.",
				MarkdownDescription: "Lease end time assigned by the orchestrator after provisioning or renewal.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				Description:         "Current slice state assigned by the orchestrator, such as Configuring, StableOK, StableError, Modifying, ModifyOK, ModifyError, AllocatedOK, or AllocatedError.",
				MarkdownDescription: "Current slice state assigned by the orchestrator. Common values include:\n\n- `Configuring` - The slice is being provisioned\n- `StableOK` - The slice is active and stable\n- `StableError` - Provisioning reached a stable state with errors\n- `Modifying` - The slice is being modified\n- `ModifyOK` - The slice modification completed\n- `ModifyError` - The slice modification failed\n- `AllocatedOK` - The slice allocation completed\n- `AllocatedError` - The slice allocation failed",
			},
			"nodes": schema.MapNestedAttribute{
				Computed:            true,
				Description:         "Map of node runtime outputs keyed by node name. Values are assigned by the orchestrator after provisioning.",
				MarkdownDescription: "Map of node runtime outputs keyed by node name. Values are assigned by the orchestrator after provisioning.",
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"management_ip":     schema.StringAttribute{Computed: true, Description: "Management IP address assigned to the node by FABRIC after provisioning.", MarkdownDescription: "Management IP address assigned to the node by FABRIC after provisioning.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"sliver_id":         schema.StringAttribute{Computed: true, Description: "FABRIC sliver identifier assigned to the node after provisioning.", MarkdownDescription: "FABRIC sliver identifier assigned to the node after provisioning.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"state":             schema.StringAttribute{Computed: true, Description: "Current node sliver state assigned by the orchestrator.", MarkdownDescription: "Current node sliver state assigned by the orchestrator.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"graph_node_id":     schema.StringAttribute{Computed: true, Description: "FIM graph node identifier assigned to the node after topology submission.", MarkdownDescription: "FIM graph node identifier assigned to the node after topology submission.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"reservation_state": schema.StringAttribute{Computed: true, Description: "Reservation state assigned to the node sliver by FABRIC.", MarkdownDescription: "Reservation state assigned to the node sliver by FABRIC.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"error_message":     schema.StringAttribute{Computed: true, Description: "Error message reported for the node sliver when provisioning or modification fails.", MarkdownDescription: "Error message reported for the node sliver when provisioning or modification fails.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"node": schema.ListNestedBlock{
				Description:         "Compute nodes to include in the FABRIC slice topology.",
				MarkdownDescription: "Compute nodes to include in the FABRIC slice topology. Each block maps to a FABRIC node sliver.",
				NestedObject: schema.NestedBlockObject{
					Attributes: nodeAttrs(),
					Blocks: map[string]schema.Block{
						"component": schema.ListNestedBlock{
							Description:         "Hardware components attached to the node, such as GPUs, SmartNICs, FPGAs, NVMe devices, or storage.",
							MarkdownDescription: "Hardware components attached to the node, such as GPUs, SmartNICs, FPGAs, NVMe devices, or storage.",
							NestedObject:        schema.NestedBlockObject{Attributes: componentAttrs()},
						},
						"storage": schema.ListNestedBlock{
							Description:         "Storage volumes requested for the node.",
							MarkdownDescription: "Storage volumes requested for the node.",
							NestedObject:        schema.NestedBlockObject{Attributes: storageAttrs()},
						},
						"route": schema.ListNestedBlock{
							Description:         "Static routes to configure in node user-data.",
							MarkdownDescription: "Static routes to configure in node user-data.",
							NestedObject:        schema.NestedBlockObject{Attributes: routeAttrs()},
						},
						"post_boot_upload": schema.ListNestedBlock{
							Description:         "Files to upload to the node after boot.",
							MarkdownDescription: "Files to upload to the node after boot.",
							NestedObject:        schema.NestedBlockObject{Attributes: postBootUploadAttrs()},
						},
					},
				},
			},
			"network": schema.ListNestedBlock{
				Description:         "Network services to include in the FABRIC slice topology.",
				MarkdownDescription: "Network services to include in the FABRIC slice topology. Each block maps to a FABRIC network service sliver.",
				NestedObject: schema.NestedBlockObject{
					Attributes: networkAttrs(),
					Blocks: map[string]schema.Block{
						"interface": schema.ListNestedBlock{
							Description:         "Node or facility-port interfaces connected to this network service.",
							MarkdownDescription: "Node or facility-port interfaces connected to this network service.",
							NestedObject: schema.NestedBlockObject{
								Attributes: interfaceAttrs(),
								Blocks: map[string]schema.Block{
									"sub_interface": schema.ListNestedBlock{
										Description:         "VLAN sub-interfaces connected to this network service interface.",
										MarkdownDescription: "VLAN sub-interfaces connected to this network service interface.",
										NestedObject:        schema.NestedBlockObject{Attributes: subInterfaceAttrs()},
									},
								},
							},
						},
					},
				},
			},
			"facility_port": schema.ListNestedBlock{
				Description:         "Facility ports to include in the slice topology for stitching to external networks.",
				MarkdownDescription: "Facility ports to include in the slice topology for stitching to external networks.",
				NestedObject: schema.NestedBlockObject{
					Attributes: facilityPortAttrs(),
					Blocks: map[string]schema.Block{
						"interface": schema.ListNestedBlock{
							Description:         "Interfaces exposed by this facility port.",
							MarkdownDescription: "Interfaces exposed by this facility port.",
							NestedObject:        schema.NestedBlockObject{Attributes: facilityPortInterfaceAttrs()},
						},
					},
				},
			},
			"switch": schema.ListNestedBlock{
				Description:         "FABRIC switch nodes to include in the slice topology.",
				MarkdownDescription: "FABRIC switch nodes to include in the slice topology.",
				NestedObject:        schema.NestedBlockObject{Attributes: switchAttrs()},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Read: true, Update: true, Delete: true}),
		},
	}
}

func nodeAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":          schema.StringAttribute{Required: true, Description: "Node name within the slice topology. Must be unique within the slice. Changing this value forces the slice to be destroyed and recreated.", MarkdownDescription: "Node name within the slice topology. Must be unique within the slice.\n\n~> **Note:** Changing this value forces the slice to be destroyed and recreated.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"site":          schema.StringAttribute{Required: true, Description: "FABRIC site code where the node is allocated, such as RENC or UKY. Changing this value forces the slice to be destroyed and recreated.", MarkdownDescription: "FABRIC site code where the node is allocated, such as `RENC` or `UKY`.\n\n~> **Note:** Changing this value forces the slice to be destroyed and recreated.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"host":          schema.StringAttribute{Optional: true, Description: "Optional FABRIC host placement hint. Sets labels.instance_parent for this node.", MarkdownDescription: "Optional FABRIC host placement hint. Sets `labels.instance_parent` for this node."},
		"instance_type": schema.StringAttribute{Optional: true, Description: "Optional FABRIC instance type name used to select node capacity from the site catalog.", MarkdownDescription: "Optional FABRIC instance type name used to select node capacity from the site catalog."},
		"image_ref":     schema.StringAttribute{Optional: true, Description: "Optional image reference to boot on the node, such as a FABRIC image name or image UUID accepted by the site.", MarkdownDescription: "Optional image reference to boot on the node, such as a FABRIC image name or image UUID accepted by the site."},
		"image_type":    schema.StringAttribute{Optional: true, Description: "Optional image type associated with image_ref, such as qcow2 when required by the selected image.", MarkdownDescription: "Optional image type associated with `image_ref`, such as `qcow2` when required by the selected image."},
		"cores":         schema.Int64Attribute{Optional: true, Description: "Optional number of CPU cores requested for the node. When omitted, FABRIC uses the instance type or site default.", MarkdownDescription: "Optional number of CPU cores requested for the node. When omitted, FABRIC uses the instance type or site default."},
		"ram":           schema.Int64Attribute{Optional: true, Description: "Optional RAM requested for the node in GB. When omitted, FABRIC uses the instance type or site default.", MarkdownDescription: "Optional RAM requested for the node in GB. When omitted, FABRIC uses the instance type or site default."},
		"disk":          schema.Int64Attribute{Optional: true, Description: "Optional root disk size requested for the node in GB. When omitted, FABRIC uses the instance type or site default.", MarkdownDescription: "Optional root disk size requested for the node in GB. When omitted, FABRIC uses the instance type or site default."},
		"boot_script": schema.StringAttribute{
			Optional:            true,
			Description:         "Inline boot script to run on first boot. Limited to 1024 characters.",
			MarkdownDescription: "Inline boot script to run on first boot. Limited to `1024` characters.",
			Validators:          []validator.String{stringvalidator.LengthAtMost(1024)},
		},
		"post_boot_execute": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			Description:         "Commands executed on the node after boot, in order. Defaults to no post-boot commands.",
			MarkdownDescription: "Commands executed on the node after boot, in order. Defaults to no post-boot commands.",
		},
		"post_update": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			Description:         "Commands run after the node package update step. Defaults to no post-update commands.",
			MarkdownDescription: "Commands run after the node package update step. Defaults to no post-update commands.",
		},
		"labels": labelsAttribute(),
	}
}

func storageAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":       schema.StringAttribute{Required: true, Description: "Storage volume name within the node.", MarkdownDescription: "Storage volume name within the node."},
		"model":      schema.StringAttribute{Optional: true, Description: "Optional FABRIC storage model to request from the catalog.", MarkdownDescription: "Optional FABRIC storage model to request from the catalog."},
		"auto_mount": schema.BoolAttribute{Optional: true, Description: "Whether to request the node-level storage flag in user-data so the volume is auto-mounted. Defaults to false.", MarkdownDescription: "Whether to request the node-level storage flag in user-data so the volume is auto-mounted. Defaults to `false`."},
	}
}

func routeAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"subnet":   schema.StringAttribute{Required: true, Description: "Destination subnet for the static route in CIDR form.", MarkdownDescription: "Destination subnet for the static route in CIDR form.", Validators: []validator.String{tfutil.CIDRStringValidator{}}},
		"next_hop": schema.StringAttribute{Required: true, Description: "Next-hop IP address for the static route.", MarkdownDescription: "Next-hop IP address for the static route.", Validators: []validator.String{tfutil.IPStringValidator{}}},
	}
}

func postBootUploadAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"local_path":  schema.StringAttribute{Required: true, Description: "Path on the machine running Terraform to upload from after the node boots.", MarkdownDescription: "Path on the machine running Terraform to upload from after the node boots."},
		"remote_path": schema.StringAttribute{Required: true, Description: "Destination path on the node for the uploaded file.", MarkdownDescription: "Destination path on the node for the uploaded file."},
	}
}

func subInterfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":      schema.StringAttribute{Required: true, Description: "Sub-interface name within the parent interface.", MarkdownDescription: "Sub-interface name within the parent interface."},
		"vlan":      schema.StringAttribute{Required: true, Description: "VLAN tag for the sub-interface in the range 0 through 4096.", MarkdownDescription: "VLAN tag for the sub-interface in the range `0` through `4096`.", Validators: []validator.String{labelStringValidator{field: "vlan"}}},
		"bandwidth": schema.Int64Attribute{Optional: true, Description: "Optional bandwidth requested for the sub-interface in Gbps.", MarkdownDescription: "Optional bandwidth requested for the sub-interface in Gbps."},
		"labels":    labelsAttribute(),
	}
}

func facilityPortAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":      schema.StringAttribute{Required: true, Description: "Facility port name from the FABRIC advertised resources.", MarkdownDescription: "Facility port name from the FABRIC advertised resources."},
		"site":      schema.StringAttribute{Required: true, Description: "FABRIC site code that advertises the facility port.", MarkdownDescription: "FABRIC site code that advertises the facility port."},
		"vlan":      schema.StringAttribute{Optional: true, Description: "VLAN tag for the facility port in the range 0 through 4096.", MarkdownDescription: "VLAN tag for the facility port in the range `0` through `4096`.", Validators: []validator.String{labelStringValidator{field: "vlan"}}},
		"bandwidth": schema.Int64Attribute{Optional: true, Description: "Optional facility-port bandwidth requested in Gbps.", MarkdownDescription: "Optional facility-port bandwidth requested in Gbps."},
		"mtu":       schema.Int64Attribute{Optional: true, Description: "Optional MTU requested for the facility port.", MarkdownDescription: "Optional MTU requested for the facility port."},
		"labels":    labelsAttribute(),
	}
}

func facilityPortInterfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":   schema.StringAttribute{Required: true, Description: "Facility-port interface name.", MarkdownDescription: "Facility-port interface name."},
		"vlan":   schema.StringAttribute{Optional: true, Description: "Optional VLAN tag for the facility-port interface in the range 0 through 4096.", MarkdownDescription: "Optional VLAN tag for the facility-port interface in the range `0` through `4096`.", Validators: []validator.String{labelStringValidator{field: "vlan"}}},
		"labels": labelsAttribute(),
	}
}

func switchAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":        schema.StringAttribute{Required: true, Description: "Switch node name within the slice topology.", MarkdownDescription: "Switch node name within the slice topology."},
		"site":        schema.StringAttribute{Required: true, Description: "FABRIC site code where the switch node is allocated.", MarkdownDescription: "FABRIC site code where the switch node is allocated."},
		"nports":      schema.Int64Attribute{Optional: true, Description: "Optional number of switch ports to request. Must be at least 1 when set.", MarkdownDescription: "Optional number of switch ports to request. Must be at least `1` when set.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"port_labels": labelsAttribute(),
	}
}

func componentAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":        schema.StringAttribute{Required: true, Description: "Component name within the node.", MarkdownDescription: "Component name within the node."},
		"type":        schema.StringAttribute{Optional: true, Description: "Component type to request. Valid values are GPU, SmartNIC, SharedNIC, FPGA, NVME, and Storage.", MarkdownDescription: "Component type to request. Valid values are:\n\n- `GPU`\n- `SmartNIC`\n- `SharedNIC`\n- `FPGA`\n- `NVME`\n- `Storage`", Validators: []validator.String{stringvalidator.OneOf("GPU", "SmartNIC", "SharedNIC", "FPGA", "NVME", "Storage")}},
		"model":       schema.StringAttribute{Optional: true, Description: "Optional component model to request from the FABRIC catalog, such as RTX6000, ConnectX-6, or P4510.", MarkdownDescription: "Optional component model to request from the FABRIC catalog, such as `RTX6000`, `ConnectX-6`, or `P4510`."},
		"fablib_name": schema.StringAttribute{Optional: true, Description: "Optional FABlib-compatible component name to expose in generated topology metadata.", MarkdownDescription: "Optional FABlib-compatible component name to expose in generated topology metadata."},
		"labels":      labelsAttribute(),
	}
}

func networkAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Required:            true,
			Description:         "Network service name within the slice topology.",
			MarkdownDescription: "Network service name within the slice topology.",
		},
		"type": schema.StringAttribute{
			Optional:            true,
			Description:         "Network service type. Valid values are L2Bridge, L2STS, L2PTP, L2Path, L2Multisite, FABNetv4, FABNetv6, FABNetv4Ext, FABNetv6Ext, L3VPN, VLAN, MPLS, and PortMirror. When omitted, the provider infers an L2 type from the connected interfaces and sites.",
			MarkdownDescription: "Network service type. When omitted, the provider infers an L2 type (`L2Bridge`, `L2STS`, or `L2PTP`) from the connected interfaces and sites. Valid values are:\n\n- `L2Bridge`\n- `L2STS`\n- `L2PTP`\n- `L2Path`\n- `L2Multisite`\n- `FABNetv4`\n- `FABNetv6`\n- `FABNetv4Ext`\n- `FABNetv6Ext`\n- `L3VPN`\n- `VLAN`\n- `MPLS`\n- `PortMirror`",
			Validators: []validator.String{stringvalidator.OneOf(
				"L2Bridge", "L2STS", "L2PTP", "L2Path", "L2Multisite",
				"FABNetv4", "FABNetv6", "FABNetv4Ext", "FABNetv6Ext",
				"L3VPN", "VLAN", "MPLS", "PortMirror",
			)},
		},
		"bandwidth":   schema.Int64Attribute{Optional: true, Description: "Optional bandwidth requested for the network service in Gbps.", MarkdownDescription: "Optional bandwidth requested for the network service in Gbps."},
		"site":        schema.StringAttribute{Optional: true, Description: "Site for single-site services. Inferred from interfaces when omitted.", MarkdownDescription: "Site for single-site services. Inferred from interfaces when omitted."},
		"technology":  schema.StringAttribute{Optional: true, Description: "Service technology hint (e.g. AL2S for L3VPN).", MarkdownDescription: "Service technology hint (e.g. `AL2S` for L3VPN)."},
		"subnet":      schema.StringAttribute{Optional: true, Description: "Subnet in CIDR form for routed services.", MarkdownDescription: "Subnet in CIDR form for routed services.", Validators: []validator.String{tfutil.CIDRStringValidator{}}},
		"mirror_from": schema.StringAttribute{Optional: true, Description: "Name of the source interface to mirror for PortMirror services.", MarkdownDescription: "Name of the source interface to mirror for `PortMirror` services."},
		"mirror_direction": schema.StringAttribute{
			Optional:            true,
			Description:         "Port-mirror direction. Valid values are Both, RX_Only, TX_Only, both, rx, and tx.",
			MarkdownDescription: "Port-mirror direction. Valid values are:\n\n- `Both`\n- `RX_Only`\n- `TX_Only`\n- `both`\n- `rx`\n- `tx`",
			Validators:          []validator.String{stringvalidator.OneOf("Both", "RX_Only", "TX_Only", "both", "rx", "tx")},
		},
		"gateway": schema.SingleNestedAttribute{
			Optional:            true,
			Description:         "Explicit gateway for routed (FABNet/L3VPN) services.",
			MarkdownDescription: "Explicit gateway for routed (FABNet/L3VPN) services.",
			Attributes: map[string]schema.Attribute{
				"ipv4":        schema.StringAttribute{Optional: true, Description: "Optional IPv4 gateway address assigned to the routed service.", MarkdownDescription: "Optional IPv4 gateway address assigned to the routed service.", Validators: []validator.String{tfutil.IPStringValidator{}}},
				"ipv4_subnet": schema.StringAttribute{Optional: true, Description: "Optional IPv4 gateway subnet in CIDR form.", MarkdownDescription: "Optional IPv4 gateway subnet in CIDR form.", Validators: []validator.String{tfutil.CIDRStringValidator{}}},
				"ipv6":        schema.StringAttribute{Optional: true, Description: "Optional IPv6 gateway address assigned to the routed service.", MarkdownDescription: "Optional IPv6 gateway address assigned to the routed service.", Validators: []validator.String{tfutil.IPStringValidator{}}},
				"ipv6_subnet": schema.StringAttribute{Optional: true, Description: "Optional IPv6 gateway subnet in CIDR form.", MarkdownDescription: "Optional IPv6 gateway subnet in CIDR form.", Validators: []validator.String{tfutil.CIDRStringValidator{}}},
				"mac":         schema.StringAttribute{Optional: true, Description: "Optional gateway MAC address assigned to the routed service.", MarkdownDescription: "Optional gateway MAC address assigned to the routed service."},
			},
		},
		"labels": labelsAttribute(),
	}
}

func interfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"node":      schema.StringAttribute{Optional: true, Description: "Name of the node whose component port this interface connects. Set node or facility.", MarkdownDescription: "Name of the node whose component port this interface connects. Set `node` or `facility`."},
		"facility":  schema.StringAttribute{Optional: true, Description: "Name of a top-level facility_port whose port this interface connects. Set node or facility.", MarkdownDescription: "Name of a top-level `facility_port` whose port this interface connects. Set `node` or `facility`."},
		"component": schema.StringAttribute{Optional: true, Description: "Optional component name on the referenced node whose port connects to this network service.", MarkdownDescription: "Optional component name on the referenced node whose port connects to this network service."},
		"port":      schema.Int64Attribute{Optional: true, Description: "Port index on the referenced node component or facility port. Defaults to 0.", MarkdownDescription: "Port index on the referenced node component or facility port. Defaults to `0`."},
		"name":      schema.StringAttribute{Optional: true, Description: "Optional interface name to use in the generated topology.", MarkdownDescription: "Optional interface name to use in the generated topology."},
		"labels":    labelsAttribute(),
	}
}
