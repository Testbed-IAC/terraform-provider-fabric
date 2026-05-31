package provider

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
				Validators:    []validator.String{fabricTimeValidator{}},
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
					},
				},
			},
			"network": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: networkAttrs(),
					Blocks: map[string]schema.Block{
						"interface": schema.ListNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: interfaceAttrs()}},
					},
				},
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
		"labels":        labelsAttribute(),
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
			Required:   true,
			Validators: []validator.String{stringvalidator.OneOf("L2Bridge", "L2STS", "L2PTP", "FABNetv4", "FABNetv6", "FABNetv4Ext", "FABNetv6Ext", "PortMirror")},
		},
		"bandwidth":   schema.Int64Attribute{Optional: true},
		"mirror_from": schema.StringAttribute{Optional: true},
		"mirror_direction": schema.StringAttribute{
			Optional:   true,
			Validators: []validator.String{stringvalidator.OneOf("Both", "RX_Only", "TX_Only")},
		},
		"labels": labelsAttribute(),
	}
}

func interfaceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"node":      schema.StringAttribute{Required: true},
		"component": schema.StringAttribute{Optional: true},
		"port":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
		"name":      schema.StringAttribute{Optional: true},
		"labels":    labelsAttribute(),
	}
}
