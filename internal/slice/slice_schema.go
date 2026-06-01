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

func sliceResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "A FABRIC slice, including compute nodes, components, and network services.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slice_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"graph_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ssh_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				DeprecationMessage:  "Use ssh_keys instead. ssh_key remains as a single-key compatibility alias for this release.",
				Description:         "Deprecated single SSH public key compatibility alias. Use ssh_keys for one or more keys. Changing this value requires replacing the slice.",
				MarkdownDescription: "Deprecated single SSH public key compatibility alias. Use `ssh_keys` for one or more keys. Changing this value requires replacing the slice.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ssh_keys": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Sensitive:           true,
				Description:         "SSH public keys to install on slice nodes. Configure exactly one of ssh_keys or the deprecated ssh_key alias. Changing this value requires replacing the slice. The provider does not store SSH key material in state after apply.",
				MarkdownDescription: "SSH public keys to install on slice nodes. Configure exactly one of `ssh_keys` or the deprecated `ssh_key` alias. Changing this value requires replacing the slice. The provider does not store SSH key material in state after apply.",
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"ssh_key_version": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"lifetime_hours": schema.Int64Attribute{
				Optional:   true,
				Computed:   true,
				Default:    int64default.StaticInt64(24),
				Validators: []validator.Int64{int64validator.AtLeast(1)},
			},
			"lease_start_time": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Validators:    []validator.String{tfutil.FabricTimeValidator{}},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"lease_end_time": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				Computed: true,
			},
			"nodes": schema.MapNestedAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"management_ip":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"sliver_id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"state":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"graph_node_id":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"reservation_state": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"error_message":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"node": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: nodeAttrs(),
					Blocks: map[string]schema.Block{
						"component": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{Attributes: componentAttrs()},
						},
						"storage": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{Attributes: storageAttrs()},
						},
						"route": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{Attributes: routeAttrs()},
						},
						"post_boot_upload": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{Attributes: postBootUploadAttrs()},
						},
					},
				},
			},
			"network": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: networkAttrs(),
					Blocks: map[string]schema.Block{
						"interface": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{
								Attributes: interfaceAttrs(),
								Blocks: map[string]schema.Block{
									"sub_interface": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{Attributes: subInterfaceAttrs()},
									},
								},
							},
						},
					},
				},
			},
			"facility_port": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: facilityPortAttrs(),
					Blocks: map[string]schema.Block{
						"interface": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{Attributes: facilityPortInterfaceAttrs()},
						},
					},
				},
			},
			"switch": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{Attributes: switchAttrs()},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Read: true, Update: true, Delete: true}),
		},
	}
}

func nodeAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":          schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"site":          schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"host":          schema.StringAttribute{Optional: true, Description: "FABRIC host placement sugar. Sets labels.instance_parent for this node.", MarkdownDescription: "FABRIC host placement sugar. Sets `labels.instance_parent` for this node."},
		"instance_type": schema.StringAttribute{Optional: true},
		"image_ref":     schema.StringAttribute{Optional: true},
		"image_type":    schema.StringAttribute{Optional: true},
		"cores":         schema.Int64Attribute{Optional: true},
		"ram":           schema.Int64Attribute{Optional: true},
		"disk":          schema.Int64Attribute{Optional: true},
		"boot_script": schema.StringAttribute{
			Optional:            true,
			Description:         "Inline boot script to run on first boot. Limited to 1024 characters.",
			MarkdownDescription: "Inline boot script to run on first boot. Limited to 1024 characters.",
			Validators:          []validator.String{stringvalidator.LengthAtMost(1024)},
		},
		"post_boot_execute": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			Description:         "Commands executed on the node after boot, in order.",
			MarkdownDescription: "Commands executed on the node after boot, in order.",
		},
		"post_update": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			Description:         "Commands run after the node's package update step.",
			MarkdownDescription: "Commands run after the node's package update step.",
		},
		"labels": labelsAttribute(),
	}
}

func storageAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":       schema.StringAttribute{Required: true},
		"model":      schema.StringAttribute{Optional: true},
		"auto_mount": schema.BoolAttribute{Optional: true, Description: "Request the node-level storage flag in user-data so the volume is auto-mounted.", MarkdownDescription: "Request the node-level storage flag in user-data so the volume is auto-mounted."},
	}
}

func routeAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"subnet":   schema.StringAttribute{Required: true, Description: "Destination subnet in CIDR form.", MarkdownDescription: "Destination subnet in CIDR form.", Validators: []validator.String{tfutil.CIDRStringValidator{}}},
		"next_hop": schema.StringAttribute{Required: true, Description: "Next-hop IP address for the route.", MarkdownDescription: "Next-hop IP address for the route.", Validators: []validator.String{tfutil.IPStringValidator{}}},
	}
}

func postBootUploadAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"local_path":  schema.StringAttribute{Required: true, Description: "Path on the machine running Terraform to upload from.", MarkdownDescription: "Path on the machine running Terraform to upload from."},
		"remote_path": schema.StringAttribute{Required: true, Description: "Destination path on the node.", MarkdownDescription: "Destination path on the node."},
	}
}

func subInterfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":      schema.StringAttribute{Required: true},
		"vlan":      schema.StringAttribute{Required: true, Description: "VLAN tag for the sub-interface (0-4096).", MarkdownDescription: "VLAN tag for the sub-interface (0-4096).", Validators: []validator.String{labelStringValidator{field: "vlan"}}},
		"bandwidth": schema.Int64Attribute{Optional: true},
		"labels":    labelsAttribute(),
	}
}

func facilityPortAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":      schema.StringAttribute{Required: true},
		"site":      schema.StringAttribute{Required: true},
		"vlan":      schema.StringAttribute{Optional: true, Description: "VLAN tag for the facility port (0-4096).", MarkdownDescription: "VLAN tag for the facility port (0-4096).", Validators: []validator.String{labelStringValidator{field: "vlan"}}},
		"bandwidth": schema.Int64Attribute{Optional: true},
		"mtu":       schema.Int64Attribute{Optional: true},
		"labels":    labelsAttribute(),
	}
}

func facilityPortInterfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":   schema.StringAttribute{Required: true},
		"vlan":   schema.StringAttribute{Optional: true, Validators: []validator.String{labelStringValidator{field: "vlan"}}},
		"labels": labelsAttribute(),
	}
}

func switchAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":        schema.StringAttribute{Required: true},
		"site":        schema.StringAttribute{Required: true},
		"nports":      schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"port_labels": labelsAttribute(),
	}
}

func componentAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":        schema.StringAttribute{Required: true},
		"type":        schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("GPU", "SmartNIC", "SharedNIC", "FPGA", "NVME", "Storage")}},
		"model":       schema.StringAttribute{Optional: true},
		"fablib_name": schema.StringAttribute{Optional: true},
		"labels":      labelsAttribute(),
	}
}

func networkAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{Required: true},
		"type": schema.StringAttribute{
			Optional:            true,
			Description:         "Network service type. When omitted, the provider infers an L2 type (L2Bridge/L2STS/L2PTP) from the connected interfaces and sites.",
			MarkdownDescription: "Network service type. When omitted, the provider infers an L2 type (`L2Bridge`/`L2STS`/`L2PTP`) from the connected interfaces and sites.",
			Validators: []validator.String{stringvalidator.OneOf(
				"L2Bridge", "L2STS", "L2PTP", "L2Path", "L2Multisite",
				"FABNetv4", "FABNetv6", "FABNetv4Ext", "FABNetv6Ext",
				"L3VPN", "VLAN", "MPLS", "PortMirror",
			)},
		},
		"bandwidth":   schema.Int64Attribute{Optional: true},
		"site":        schema.StringAttribute{Optional: true, Description: "Site for single-site services. Inferred from interfaces when omitted.", MarkdownDescription: "Site for single-site services. Inferred from interfaces when omitted."},
		"technology":  schema.StringAttribute{Optional: true, Description: "Service technology hint (e.g. AL2S for L3VPN).", MarkdownDescription: "Service technology hint (e.g. `AL2S` for L3VPN)."},
		"subnet":      schema.StringAttribute{Optional: true, Description: "Subnet in CIDR form for routed services.", MarkdownDescription: "Subnet in CIDR form for routed services.", Validators: []validator.String{tfutil.CIDRStringValidator{}}},
		"mirror_from": schema.StringAttribute{Optional: true},
		"mirror_direction": schema.StringAttribute{
			Optional:            true,
			Description:         "Port-mirror direction. Accepts Both/RX_Only/TX_Only or the lowercase aliases both/rx/tx.",
			MarkdownDescription: "Port-mirror direction. Accepts `Both`/`RX_Only`/`TX_Only` or the lowercase aliases `both`/`rx`/`tx`.",
			Validators:          []validator.String{stringvalidator.OneOf("Both", "RX_Only", "TX_Only", "both", "rx", "tx")},
		},
		"gateway": schema.SingleNestedAttribute{
			Optional:            true,
			Description:         "Explicit gateway for routed (FABNet/L3VPN) services.",
			MarkdownDescription: "Explicit gateway for routed (FABNet/L3VPN) services.",
			Attributes: map[string]schema.Attribute{
				"ipv4":        schema.StringAttribute{Optional: true, Validators: []validator.String{tfutil.IPStringValidator{}}},
				"ipv4_subnet": schema.StringAttribute{Optional: true, Validators: []validator.String{tfutil.CIDRStringValidator{}}},
				"ipv6":        schema.StringAttribute{Optional: true, Validators: []validator.String{tfutil.IPStringValidator{}}},
				"ipv6_subnet": schema.StringAttribute{Optional: true, Validators: []validator.String{tfutil.CIDRStringValidator{}}},
				"mac":         schema.StringAttribute{Optional: true},
			},
		},
		"labels": labelsAttribute(),
	}
}

func interfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"node":      schema.StringAttribute{Optional: true, Description: "Name of the node whose component port this interface connects. Set node or facility.", MarkdownDescription: "Name of the node whose component port this interface connects. Set `node` or `facility`."},
		"facility":  schema.StringAttribute{Optional: true, Description: "Name of a top-level facility_port whose port this interface connects. Set node or facility.", MarkdownDescription: "Name of a top-level `facility_port` whose port this interface connects. Set `node` or `facility`."},
		"component": schema.StringAttribute{Optional: true},
		"port":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
		"name":      schema.StringAttribute{Optional: true},
		"labels":    labelsAttribute(),
	}
}
