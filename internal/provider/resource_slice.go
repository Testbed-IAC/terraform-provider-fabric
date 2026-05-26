package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/permission"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/poller"
)

type SliceResource struct {
	client      fabricclient.FabricClient
	projectTags map[string]bool
}

func NewSliceResource() resource.Resource {
	return &SliceResource{}
}

func (r *SliceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slice"
}

func (r *SliceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = sliceResourceSchema(ctx)
}

func (r *SliceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return
	}
	r.client = data.client
	r.projectTags = data.projectTags
}

func (r *SliceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SliceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	permission.Validate(ctx, permissionRequest(plan, r.projectTags), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := validateCatalog(plan); err != nil {
		resp.Diagnostics.AddError("Invalid FABRIC catalog entry", err.Error())
		return
	}
	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		addBuildError(&resp.Diagnostics, err)
		return
	}
	tflog.Info(ctx, "creating FABRIC slice", map[string]any{"slice_name": plan.Name.ValueString(), "node_count": len(plan.Nodes), "network_count": len(plan.Networks)})
	tflog.Trace(ctx, "sending FABRIC GraphML", map[string]any{"graphml_bytes": len(graphML), "graphml_preview": preview(graphML)})
	slivers, err := r.client.CreateSlice(ctx, plan.Name.ValueString(), graphML, []string{plan.SSHKey.ValueString()}, fabricclient.CreateOpts{
		LifetimeHours:  int32(int64Value(plan.LifetimeHours)),
		LeaseStartTime: stringValue(plan.LeaseStartTime),
	})
	if err != nil {
		tflog.Error(ctx, "create slice failed", map[string]any{"err": err})
		resp.Diagnostics.AddError("Create FABRIC slice failed", err.Error())
		return
	}
	if len(slivers) == 0 {
		resp.Diagnostics.AddError("Create FABRIC slice failed", "The orchestrator returned no slivers, so the slice id is unknown.")
		return
	}
	sliceID := slivers[0].SliceID
	plan.ID = types.StringValue(sliceID)
	plan.SliceID = types.StringValue(sliceID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SetField(ctx, "slice_id", sliceID)
	timeout, diags := plan.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	final, err := poller.WaitForSlice(ctx, r.client, sliceID, []string{"StableOK"}, []string{"StableError", "AllocatedError", "Dead", "ClosingError"}, timeout, 15*time.Second)
	if err != nil {
		tflog.Error(ctx, "slice did not become stable", map[string]any{"err": err})
		resp.Diagnostics.AddError("FABRIC slice did not become stable", err.Error())
		return
	}
	if err := r.refreshFromSlice(ctx, final, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Refresh FABRIC slice failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SliceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SliceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sliceID := stringValue(state.SliceID)
	if sliceID == "" {
		sliceID = stringValue(state.ID)
	}
	slice, err := r.client.GetSlice(ctx, sliceID)
	if errors.Is(err, fabricclient.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC slice failed", err.Error())
		return
	}
	switch slice.State {
	case "Dead":
		resp.State.RemoveResource(ctx)
		return
	case "Closing", "ClosingError":
		tflog.Warn(ctx, "slice is closing and will be removed from state", map[string]any{"slice_id": sliceID, "state": slice.State})
		resp.Diagnostics.AddWarning("FABRIC slice is closing", "The slice is in "+slice.State+" and has been removed from Terraform state.")
		resp.State.RemoveResource(ctx)
		return
	case "StableError", "ModifyError":
		resp.Diagnostics.AddWarning("FABRIC slice in error state", slice.Notice)
	case "ModifyOK":
		tflog.Warn(ctx, "recovering slice stuck in ModifyOK", map[string]any{"slice_id": sliceID, "previous_state": slice.State})
		accepted, err := r.client.AcceptModify(ctx, sliceID)
		if err != nil {
			resp.Diagnostics.AddError("Accept FABRIC modify failed", err.Error())
			return
		}
		slice = accepted
	}
	if err := r.refreshFromSlice(ctx, slice, &state, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Read FABRIC slice failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SliceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SliceResourceModel
	var state SliceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sliceID := state.SliceID.ValueString()
	permission.Validate(ctx, permissionRequest(plan, r.projectTags), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if cur, err := r.client.GetSlice(ctx, sliceID); err == nil && cur != nil && cur.State == "ModifyOK" {
		tflog.Warn(ctx, "recovering previous ModifyOK before update", map[string]any{"slice_id": sliceID, "previous_state": cur.State})
		if _, err := r.client.AcceptModify(ctx, sliceID); err != nil {
			resp.Diagnostics.AddError("Accept previous FABRIC modify failed", err.Error())
			return
		}
	}
	if topologyEquivalent(ctx, state, plan) && state.LifetimeHours.ValueInt64() != plan.LifetimeHours.ValueInt64() {
		leaseEnd := stringValue(plan.LeaseEndTime)
		if leaseEnd == "" {
			leaseEnd = time.Now().UTC().Add(time.Duration(plan.LifetimeHours.ValueInt64()) * time.Hour).Format(time.RFC3339)
		}
		if err := r.client.RenewSlice(ctx, sliceID, leaseEnd); err != nil {
			resp.Diagnostics.AddError("Renew FABRIC slice failed", err.Error())
			return
		}
		slice, err := r.client.GetSlice(ctx, sliceID)
		if err != nil {
			resp.Diagnostics.AddError("Read renewed FABRIC slice failed", err.Error())
			return
		}
		if err := r.refreshFromSlice(ctx, slice, &plan, &resp.Diagnostics); err != nil {
			resp.Diagnostics.AddError("Refresh FABRIC slice failed", err.Error())
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	if err := validateCatalog(plan); err != nil {
		resp.Diagnostics.AddError("Invalid FABRIC catalog entry", err.Error())
		return
	}
	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		addBuildError(&resp.Diagnostics, err)
		return
	}
	tflog.Info(ctx, "updating FABRIC slice", map[string]any{"slice_id": sliceID})
	if _, err := r.client.ModifySlice(ctx, sliceID, graphML); err != nil {
		resp.Diagnostics.AddError("Modify FABRIC slice failed", err.Error())
		return
	}
	timeout, diags := plan.Timeouts.Update(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	final, err := poller.WaitForSlice(ctx, r.client, sliceID, []string{"ModifyOK", "ModifyError", "StableOK"}, []string{"Dead", "ClosingError"}, timeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddError("FABRIC modify did not complete", err.Error())
		return
	}
	if final.State == "ModifyError" {
		tflog.Warn(ctx, "FABRIC modify reached ModifyError; accepting to prune failures", map[string]any{"slice_id": sliceID, "notice": final.Notice})
		resp.Diagnostics.AddWarning("FABRIC modify partially failed", final.Notice)
	}
	accepted, err := r.client.AcceptModify(ctx, sliceID)
	if err != nil {
		resp.Diagnostics.AddError("Accept FABRIC modify failed", err.Error())
		return
	}
	if err := r.refreshFromSlice(ctx, accepted, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Refresh FABRIC slice failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SliceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SliceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sliceID := state.SliceID.ValueString()
	tflog.Info(ctx, "deleting FABRIC slice", map[string]any{"slice_id": sliceID})
	err := r.client.DeleteSlice(ctx, sliceID)
	if errors.Is(err, fabricclient.ErrNotFound) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Delete FABRIC slice failed", err.Error())
		return
	}
	timeout, diags := state.Timeouts.Delete(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := poller.WaitForSlice(ctx, r.client, sliceID, []string{"Dead", "ClosingError"}, nil, timeout, 15*time.Second); err != nil {
		resp.Diagnostics.AddError("FABRIC slice deletion did not complete", err.Error())
	}
}

func (r *SliceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("slice_id"), req.ID)...)
}

func (r *SliceResource) refreshFromSlice(ctx context.Context, slice *fabricclient.Slice, state *SliceResourceModel, diags *diag.Diagnostics) error {
	if slice == nil {
		return nil
	}
	state.ID = types.StringValue(slice.SliceID)
	state.SliceID = types.StringValue(slice.SliceID)
	state.Name = types.StringValue(slice.Name)
	state.GraphID = types.StringValue(slice.GraphID)
	state.State = types.StringValue(slice.State)
	state.LeaseStartTime = types.StringValue(slice.LeaseStartTime)
	state.LeaseEndTime = types.StringValue(slice.LeaseEndTime)
	if slice.Model != "" {
		actual, err := topology.Load(strings.NewReader(slice.Model))
		if err != nil {
			return fmt.Errorf("loading returned topology: %w", err)
		}
		desired, _, err := buildTopology(ctx, *state)
		if err == nil {
			diff := topology.DiffTopologies(desired, actual)
			if diff.HasChanges() {
				tflog.Info(ctx, "FABRIC topology drift detected", map[string]any{"slice_id": slice.SliceID, "diff_summary": diff.Summary()})
				for _, d := range diff.Diagnostics() {
					diags.AddWarning("Topology drift: "+d.Field(), d.Suggestion())
				}
			}
		}
		if err := r.setNodeOutputs(ctx, slice.SliceID, actual, state); err != nil {
			return err
		}
	}
	return nil
}

func (r *SliceResource) setNodeOutputs(ctx context.Context, sliceID string, topo *topology.Topology, state *SliceResourceModel) error {
	outputs := map[string]NodeOutputModel{}
	slivers, err := r.client.GetSlivers(ctx, sliceID)
	if err != nil {
		tflog.Warn(ctx, "could not fetch slivers", map[string]any{"slice_id": sliceID, "err": err})
	}
	byGraphNode := map[string]fabricclient.Sliver{}
	for _, sl := range slivers {
		byGraphNode[sl.GraphNodeID] = sl
	}
	for _, n := range state.Nodes {
		node, ok := topo.Node(n.Name.ValueString())
		if !ok {
			tflog.Warn(ctx, "node missing in returned topology", map[string]any{"node": n.Name.ValueString(), "slice_id": sliceID})
			continue
		}
		nodeSliver, err := node.Sliver()
		if err != nil {
			return fmt.Errorf("reading node sliver %s: %w", n.Name.ValueString(), err)
		}
		out := NodeOutputModel{
			ManagementIP: types.StringValue(nodeSliver.MgmtIP),
			GraphNodeID:  types.StringValue(node.ID()),
		}
		if nodeSliver.ReservationInfo != nil {
			out.SliverID = types.StringValue(nodeSliver.ReservationInfo.ReservationID)
			out.ReservationState = types.StringValue(nodeSliver.ReservationInfo.ReservationState)
			out.ErrorMessage = types.StringValue(nodeSliver.ReservationInfo.ErrorMessage)
		}
		if sl, ok := byGraphNode[node.ID()]; ok {
			out.State = types.StringValue(sl.State)
			if out.SliverID.IsNull() || out.SliverID.ValueString() == "" {
				out.SliverID = types.StringValue(sl.SliverID)
			}
		}
		outputs[n.Name.ValueString()] = out
	}
	value, diags := types.MapValueFrom(ctx, types.ObjectType{AttrTypes: nodeOutputAttrTypes()}, outputs)
	if diags.HasError() {
		return fmt.Errorf("building node outputs: %s", diags[0].Summary())
	}
	state.NodeOutputs = value
	return nil
}

func topologyEquivalent(ctx context.Context, a, b SliceResourceModel) bool {
	_, graphA, errA := buildTopology(ctx, a)
	_, graphB, errB := buildTopology(ctx, b)
	return errA == nil && errB == nil && graphA == graphB
}

func addBuildError(diags *diag.Diagnostics, err error) {
	diags.AddError("Build FABRIC topology failed", err.Error())
}

func preview(s string) string {
	if len(s) <= 500 {
		return s
	}
	return s[:500]
}
