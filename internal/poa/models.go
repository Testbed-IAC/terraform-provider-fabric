package poa

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ResourceModel is the Terraform state model for a fabric_poa resource. It holds
// the request arguments for one perform-operational-action call and the terminal
// state, error, and info the orchestrator returns.
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

// VCPUCPUModel maps one vcpu_cpu_map entry to a guest-vCPU-to-host-CPU pin used
// by the cpupin and numatune operations.
type VCPUCPUModel struct {
	VCPU types.String `tfsdk:"vcpu"`
	CPU  types.String `tfsdk:"cpu"`
}

// KeyModel maps one keys entry to an SSH public key and comment used by the
// addkey and removekey operations.
type KeyModel struct {
	Key     types.String `tfsdk:"key"`
	Comment types.String `tfsdk:"comment"`
}
