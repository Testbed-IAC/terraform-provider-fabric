package provider

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

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/poller"
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
	poaPollInterval       = 15 * time.Second
)

// POAResource runs a FABRIC perform-operational-action request.
type POAResource struct {
	client fabricclient.FabricClient
}

// NewPOAResource returns the FABRIC POA action resource.
func NewPOAResource() resource.Resource {
	return &POAResource{}
}

func (r *POAResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_poa"
}

func (r *POAResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Runs a FABRIC perform-operational-action request against a sliver. This is an action resource: replacing it re-runs the operation, and delete is non-reversible/no-op.",
		MarkdownDescription: "Runs a FABRIC perform-operational-action request against a sliver. This is an action resource: replacing it re-runs the operation, and delete is non-reversible/no-op.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, Description: "POA identifier.", MarkdownDescription: "POA identifier.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"poa_id":    schema.StringAttribute{Computed: true, Description: "FABRIC POA identifier.", MarkdownDescription: "FABRIC POA identifier.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"sliver_id": schema.StringAttribute{Required: true, Description: "Sliver identifier to run the POA against. Changing this value replaces the action resource and re-runs the operation.", MarkdownDescription: "Sliver identifier to run the POA against. Changing this value replaces the action resource and re-runs the operation.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"operation": schema.StringAttribute{
				Required:            true,
				Description:         "POA operation to run. Supported values are cpuinfo, numainfo, cpupin, numatune, reboot, addkey, removekey, and rescan. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "POA operation to run. Supported values are `cpuinfo`, `numainfo`, `cpupin`, `numatune`, `reboot`, `addkey`, `removekey`, and `rescan`. Changing this value replaces the action resource and re-runs the operation.",
				Validators:          []validator.String{stringvalidator.OneOf(poaOperationCPUInfo, poaOperationNUMAInfo, poaOperationCPUPin, poaOperationNUMATune, poaOperationReboot, poaOperationAddKey, poaOperationRemoveKey, poaOperationRescan)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"node_set": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Optional node set for operations that accept node targets. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional node set for operations that accept node targets. Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"bdf": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Optional PCI BDF list for operations that accept device targets. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional PCI BDF list for operations that accept device targets. Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Arbitrary trigger values. Changing this map replaces the action resource and re-runs the POA.",
				MarkdownDescription: "Arbitrary trigger values. Changing this map replaces the action resource and re-runs the POA.",
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"state": schema.StringAttribute{Computed: true, Description: "Terminal or current POA state.", MarkdownDescription: "Terminal or current POA state."},
			"error": schema.StringAttribute{Computed: true, Description: "POA error text when the operation fails.", MarkdownDescription: "POA error text when the operation fails."},
			"info":  schema.StringAttribute{Computed: true, Description: "POA info payload encoded as JSON when returned by FABRIC.", MarkdownDescription: "POA info payload encoded as JSON when returned by FABRIC."},
			"vcpu_cpu_map": schema.ListNestedAttribute{
				Optional:            true,
				Description:         "Optional vCPU to CPU pinning map. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional vCPU to CPU pinning map. Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"vcpu": schema.StringAttribute{Required: true, Description: "Guest vCPU identifier.", MarkdownDescription: "Guest vCPU identifier."},
					"cpu":  schema.StringAttribute{Required: true, Description: "Host CPU identifier.", MarkdownDescription: "Host CPU identifier."},
				}},
			},
			"keys": schema.ListNestedAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "Optional SSH keys for addkey/removekey operations. Changing this value replaces the action resource and re-runs the operation.",
				MarkdownDescription: "Optional SSH keys for addkey/removekey operations. Changing this value replaces the action resource and re-runs the operation.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"key":     schema.StringAttribute{Required: true, Sensitive: true, Description: "SSH public key content.", MarkdownDescription: "SSH public key content."},
					"comment": schema.StringAttribute{Required: true, Description: "SSH key comment.", MarkdownDescription: "SSH key comment."},
				}},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Read: true}),
		},
	}
}

func (r *POAResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*FabricProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return
	}
	r.client = data.Client
}

func (r *POAResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan POAResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request := poaRequestFromModel(plan)
	poa, err := r.client.CreatePOA(ctx, stringValue(plan.SliverID), request)
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
	final, err := poller.WaitForPOA(ctx, r.client, poa.POAID, timeout, poaPollInterval)
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

func (r *POAResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state POAResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	poaID := stringValue(state.POAID)
	if poaID == "" {
		poaID = stringValue(state.ID)
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
		poa, err = poller.WaitForPOA(ctx, r.client, poaID, timeout, poaPollInterval)
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

func (r *POAResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update FABRIC POA unsupported", "POA resources are action resources. Change a RequiresReplace argument or triggers to run the operation again.")
}

func (r *POAResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state POAResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "forgetting non-reversible FABRIC POA action", map[string]any{"poa_id": stringValue(state.POAID), "operation": stringValue(state.Operation)})
}

func poaRequestFromModel(model POAResourceModel) fabricclient.POARequest {
	request := fabricclient.POARequest{
		Operation: stringValue(model.Operation),
		NodeSet:   stringValues(model.NodeSet),
		BDF:       stringValues(model.BDF),
	}
	for _, mapping := range model.VCPUCPUMap {
		request.VCPUCPUMap = append(request.VCPUCPUMap, fabricclient.POAVCPUCPU{VCPU: stringValue(mapping.VCPU), CPU: stringValue(mapping.CPU)})
	}
	for _, key := range model.Keys {
		request.Keys = append(request.Keys, fabricclient.POAKey{Key: stringValue(key.Key), Comment: stringValue(key.Comment)})
	}
	return request
}

func stringValues(values []types.String) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, stringValue(value))
	}
	return out
}

func updatePOAState(model *POAResourceModel, poa *fabricclient.POA) {
	if poa == nil {
		return
	}
	model.ID = types.StringValue(poa.POAID)
	model.POAID = types.StringValue(poa.POAID)
	model.State = types.StringValue(poa.State)
	model.Error = types.StringValue(poa.Error)
	model.Info = types.StringValue(poa.InfoJSON)
}
