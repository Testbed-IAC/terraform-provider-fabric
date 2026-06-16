// Package poa implements the fabric_poa Terraform resource: a one-shot action
// resource that submits a FABRIC perform-operational-action request against a
// sliver and waits for it to reach a terminal state.
package poa

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/poller"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

const (
	poaOperationCPUInfo   = "cpuinfo"
	poaOperationNUMAInfo  = "numainfo"
	poaOperationCPUPin    = "cpupin"
	poaOperationNUMATune  = "numatune"
	poaOperationReboot    = "reboot"
	poaOperationAddKey    = "addkey"
	poaOperationRemoveKey = "removekey"
	poaOperationRescan    = "rescan"
	poaDefaultTimeout     = 10 * time.Minute
	// poaPollInterval is the default POA poll cadence; FABRIC_POLL_INTERVAL
	// overrides it (see tfutil.PollInterval), chiefly for testmode acceptance runs.
	poaPollInterval = 15 * time.Second
)

// POAResource runs a FABRIC perform-operational-action request.
type POAResource struct {
	client fabricclient.API
}

// NewResource returns the FABRIC POA action resource.
func NewResource() resource.Resource {
	return &POAResource{}
}

func (r *POAResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_poa"
}

func (r *POAResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Runs a FABRIC perform-operational-action request against a sliver. This is an action resource: replacing it re-runs the operation, and delete is non-reversible/no-op.",
		MarkdownDescription: "Runs a FABRIC perform-operational-action request against a sliver. This is an action resource: replacing it re-runs the operation, and delete is non-reversible/no-op.\n\n~> **Note:** Changing any request argument forces this resource to be replaced and re-runs the operation.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, Description: "POA identifier assigned by FABRIC after the operation is created.", MarkdownDescription: "POA identifier assigned by FABRIC after the operation is created.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"poa_id":    schema.StringAttribute{Computed: true, Description: "FABRIC POA identifier assigned by FABRIC after the operation is created.", MarkdownDescription: "FABRIC POA identifier assigned by FABRIC after the operation is created.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"sliver_id": schema.StringAttribute{Required: true, Description: "Sliver identifier to run the POA against. Changing this value replaces the action resource and re-runs the operation.", MarkdownDescription: "Sliver identifier to run the POA against.\n\n~> **Note:** Changing this value replaces the action resource and re-runs the operation.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"operation": schema.StringAttribute{
				Required:            true,
				Description:         "POA operation to run. Supported values are cpuinfo, numainfo, cpupin, numatune, reboot, addkey, removekey, and rescan. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "POA operation to run. Supported values are:\n\n- `cpuinfo`\n- `numainfo`\n- `cpupin`\n- `numatune`\n- `reboot`\n- `addkey`\n- `removekey`\n- `rescan`\n\n~> **Note:** Changing this value replaces the action resource and re-runs the operation.",
				Validators:          []validator.String{stringvalidator.OneOf(poaOperationCPUInfo, poaOperationNUMAInfo, poaOperationCPUPin, poaOperationNUMATune, poaOperationReboot, poaOperationAddKey, poaOperationRemoveKey, poaOperationRescan)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"node_set": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Optional node set for operations that accept node targets. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional node set for operations that accept node targets.\n\n~> **Note:** Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"bdf": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Optional PCI bus-device-function list for operations that accept device targets. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional PCI bus-device-function list for operations that accept device targets.\n\n~> **Note:** Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Arbitrary trigger values. Changing this map replaces the action resource and re-runs the POA.",
				MarkdownDescription: "Arbitrary trigger values.\n\n~> **Note:** Changing this map replaces the action resource and re-runs the POA.",
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"state": schema.StringAttribute{Computed: true, Description: "Terminal or current POA state assigned by FABRIC.", MarkdownDescription: "Terminal or current POA state assigned by FABRIC."},
			"error": schema.StringAttribute{Computed: true, Description: "POA error text assigned by FABRIC when the operation fails.", MarkdownDescription: "POA error text assigned by FABRIC when the operation fails."},
			"info":  schema.StringAttribute{Computed: true, Description: "POA info payload encoded as JSON when returned by FABRIC.", MarkdownDescription: "POA info payload encoded as JSON when returned by FABRIC."},
			"vcpu_cpu_map": schema.ListNestedAttribute{
				Optional:            true,
				Description:         "Optional vCPU to CPU pinning map for cpupin and numatune operations. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional vCPU to CPU pinning map for `cpupin` and `numatune` operations.\n\n~> **Note:** Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"vcpu": schema.StringAttribute{Required: true, Description: "Guest vCPU identifier to pin.", MarkdownDescription: "Guest vCPU identifier to pin."},
					"cpu":  schema.StringAttribute{Required: true, Description: "Host CPU identifier to pin the vCPU to.", MarkdownDescription: "Host CPU identifier to pin the vCPU to."},
				}},
			},
			"keys": schema.ListNestedAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "Optional SSH keys for addkey and removekey operations. Changing this value replaces the action resource and re-runs the operation. These values are masked in plan output and state.",
				MarkdownDescription: "Optional SSH keys for `addkey` and `removekey` operations. These values are masked in plan output and state.\n\n~> **Note:** Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"key":     schema.StringAttribute{Required: true, Sensitive: true, Description: "SSH public key content. This value is masked in plan output and state.", MarkdownDescription: "SSH public key content. This value is masked in plan output and state."},
					"comment": schema.StringAttribute{Required: true, Description: "SSH key comment associated with the key.", MarkdownDescription: "SSH key comment associated with the key."},
				}},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Read: true}),
		},
	}
}

func (r *POAResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := providercfg.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if data == nil {
		return
	}
	r.client = data.Client
}

func (r *POAResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request := poaRequestFromModel(plan)
	poa, err := r.client.CreatePOA(ctx, tfutil.StringValue(plan.SliverID), request)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("sliver_id"), "Create FABRIC POA failed", "The FABRIC orchestrator rejected the POA request. Correct the sliver_id and operation arguments, then apply again. Original error: "+err.Error())
		return
	}
	if poa == nil || poa.POAID == "" {
		resp.Diagnostics.AddError("Create FABRIC POA failed", "The orchestrator did not return a POA identifier, so the operation cannot be tracked.")
		return
	}
	updatePOAState(&plan, poa)
	timeout, diags := plan.Timeouts.Create(ctx, poaDefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	final, err := poller.WaitForPOA(ctx, r.client, poa.POAID, timeout, tfutil.PollInterval(poaPollInterval))
	if final != nil {
		updatePOAState(&plan, final)
	}
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("FABRIC POA failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read re-polls WaitForPOA when the stored POA is non-terminal, and surfaces a
// terminal Failed state as a warning rather than an error so refresh does not fail
// the plan.
func (r *POAResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	poaID := tfutil.StringValue(state.POAID)
	if poaID == "" {
		poaID = tfutil.StringValue(state.ID)
	}
	if poaID == "" {
		return
	}
	poa, err := r.client.GetPOA(ctx, poaID)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("poa_id"), "Read FABRIC POA failed", "The FABRIC orchestrator could not read the POA status. Refresh again or recreate the action resource. Original error: "+err.Error())
		return
	}
	if poa != nil && poa.State != poller.POASuccessState && poa.State != poller.POAFailedState {
		timeout, diags := state.Timeouts.Read(ctx, poaDefaultTimeout)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		poa, err = poller.WaitForPOA(ctx, r.client, poaID, timeout, tfutil.PollInterval(poaPollInterval))
		if err != nil && poa == nil {
			resp.Diagnostics.AddError("Read FABRIC POA failed", err.Error())
			return
		}
	}
	updatePOAState(&state, poa)
	if state.State.ValueString() == poller.POAFailedState {
		resp.Diagnostics.AddWarning("FABRIC POA failed", state.Error.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *POAResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update FABRIC POA unsupported", "POA resources are action resources. Change a RequiresReplace argument or triggers to run the operation again.")
}

func (r *POAResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "forgetting non-reversible FABRIC POA action", map[string]any{"poa_id": tfutil.StringValue(state.POAID), "operation": tfutil.StringValue(state.Operation)})
}

func poaRequestFromModel(model ResourceModel) fabricclient.POARequest {
	request := fabricclient.POARequest{
		Operation: tfutil.StringValue(model.Operation),
		NodeSet:   tfutil.StringValues(model.NodeSet),
		BDF:       tfutil.StringValues(model.BDF),
	}
	for _, mapping := range model.VCPUCPUMap {
		request.VCPUCPUMap = append(request.VCPUCPUMap, fabricclient.POAVCPUCPU{VCPU: tfutil.StringValue(mapping.VCPU), CPU: tfutil.StringValue(mapping.CPU)})
	}
	for _, key := range model.Keys {
		request.Keys = append(request.Keys, fabricclient.POAKey{Key: tfutil.StringValue(key.Key), Comment: tfutil.StringValue(key.Comment)})
	}
	return request
}

func updatePOAState(model *ResourceModel, poa *fabricclient.POA) {
	if poa == nil {
		return
	}
	model.ID = types.StringValue(poa.POAID)
	model.POAID = types.StringValue(poa.POAID)
	model.State = types.StringValue(poa.State)
	model.Error = types.StringValue(poa.Error)
	model.Info = types.StringValue(poa.InfoJSON)
}
