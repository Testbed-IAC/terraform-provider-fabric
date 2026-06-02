package poa

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ResourceModel struct {
	ID         types.String   `tfsdk:"id"`
	POAID      types.String   `tfsdk:"poa_id"`
	SliverID   types.String   `tfsdk:"sliver_id"`
	Operation  types.String   `tfsdk:"operation"`
	VCPUCPUMap []VCPUCPUModel `tfsdk:"vcpu_cpu_map"`
	NodeSet    []types.String `tfsdk:"node_set"`
	BDF        []types.String `tfsdk:"bdf"`
	Keys       []KeyModel     `tfsdk:"keys"`
	Triggers   types.Map      `tfsdk:"triggers"`
	State      types.String   `tfsdk:"state"`
	Error      types.String   `tfsdk:"error"`
	Info       types.String   `tfsdk:"info"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}

type VCPUCPUModel struct {
	VCPU types.String `tfsdk:"vcpu"`
	CPU  types.String `tfsdk:"cpu"`
}

type KeyModel struct {
	Key     types.String `tfsdk:"key"`
	Comment types.String `tfsdk:"comment"`
}
