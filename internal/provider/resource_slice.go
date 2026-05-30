package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
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
	client          fabricclient.FabricClient
	tokenSource     fabricclient.TokenSource
	resourcesSource resourcesSummarySource
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
	data, ok := req.ProviderData.(*FabricProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return
	}
	r.client = data.Client
	r.tokenSource = data.TokenSource
	r.resourcesSource = data.ResourcesSource
}

func (r *SliceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan SliceResourceModel
	var config SliceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	permission.Validate(ctx, permissionRequest(plan), r.tokenSource, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := validateCatalog(plan); err != nil {
		tflog.Error(ctx, "invalid FABRIC catalog entry in plan", map[string]any{"error": err.Error()})
		resp.Diagnostics.AddError(
			"Invalid FABRIC catalog entry",
			"Terraform could not match an instance type or component model against the embedded FABRIC catalog. Correct the instance_type, component type/model, or fablib_name and run plan again. Original error: "+err.Error(),
		)
		return
	}
	validateTopology(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	validateResourcesSummary(ctx, plan, r.resourcesSource, &resp.Diagnostics)
}

func (r *SliceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SliceResourceModel
	var config SliceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		addBuildError(&resp.Diagnostics, err)
		return
	}
	tflog.Info(ctx, "creating FABRIC slice", map[string]any{"slice_name": plan.Name.ValueString(), "node_count": len(plan.Nodes), "network_count": len(plan.Networks)})
	tflog.Trace(ctx, "sending FABRIC GraphML", map[string]any{"graphml_bytes": len(graphML), "graphml_preview": preview(graphML)})
	slivers, err := r.client.CreateSlice(ctx, plan.Name.ValueString(), graphML, []string{config.SSHKey.ValueString()}, fabricclient.CreateOpts{
		LifetimeHours:  int32(int64Value(plan.LifetimeHours)),
		LeaseStartTime: stringValue(plan.LeaseStartTime),
	})
	if err != nil {
		tflog.Error(ctx, "create slice failed", map[string]any{"err": err.Error()})
		r.addClientError(&resp.Diagnostics, "Create FABRIC slice failed", err)
		return
	}
	if len(slivers) == 0 {
		resp.Diagnostics.AddError("Create FABRIC slice failed", "The orchestrator returned no slivers, so the slice id is unknown.")
		return
	}
	sliceID := slivers[0].SliceID
	plan.ID = types.StringValue(sliceID)
	plan.SliceID = types.StringValue(sliceID)
	plan.SSHKey = types.StringNull()
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
	plan.SSHKey = types.StringNull()
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
		r.addClientError(&resp.Diagnostics, "Read FABRIC slice failed", err)
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
		plan.SSHKey = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		addBuildError(&resp.Diagnostics, err)
		return
	}
	tflog.Info(ctx, "updating FABRIC slice", map[string]any{"slice_id": sliceID})
	if _, err := r.client.ModifySlice(ctx, sliceID, graphML); err != nil {
		r.addClientError(&resp.Diagnostics, "Modify FABRIC slice failed", err)
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
	plan.SSHKey = types.StringNull()
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
	if state.LeaseStartTime.IsNull() || state.LeaseStartTime.IsUnknown() || state.LeaseStartTime.ValueString() == "" {
		state.LeaseStartTime = types.StringValue(slice.LeaseStartTime)
	}
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
	return s[:500] + "...(truncated)"
}

// addClientError translates a known fabricclient error into an actionable
// diagnostic. The defaultSummary is used for errors that do not match a known
// sentinel.
func (r *SliceResource) addClientError(diags *diag.Diagnostics, defaultSummary string, err error) {
	// Policy/PDP violations come back from the orchestrator as HTTP 500 bodies
	// containing "PDP Failure" / "Policy Violation". They are really 403-style
	// authorization failures and deserve a tag-aware diagnostic before falling
	// through to the generic ErrServerError mapping.
	if summary, detail, ok := r.pdpDiagnostic(err); ok {
		diags.AddError(summary, detail)
		return
	}

	summary := defaultSummary
	detail := err.Error()

	switch {
	case errors.Is(err, fabricclient.ErrUnauthorized):
		summary = "FABRIC Authentication Failed"
		detail = "The orchestrator rejected the request with 401 Unauthorized. " +
			"Your token may have expired (FABRIC tokens are valid for ~1 hour). " +
			"Get a fresh token from https://portal.fabric-testbed.net → Experiments → Tokens. " +
			"Original error: " + err.Error()
	case errors.Is(err, fabricclient.ErrForbidden):
		summary = "FABRIC Authorization Failed"
		detail = fmt.Sprintf("The orchestrator rejected the request with 403 Forbidden. "+
			"Verify project %q and that your project has the required permission tags. "+
			"Original error: %s", r.projectName(), err.Error())
	case errors.Is(err, fabricclient.ErrBadRequest):
		summary = "Invalid Slice Configuration"
		detail = "The orchestrator rejected the GraphML topology as invalid. " +
			"This may indicate a bug in the provider's topology builder. " +
			"Original error: " + err.Error()
	case errors.Is(err, fabricclient.ErrServerError):
		summary = "FABRIC Orchestrator Error"
		detail = "The orchestrator returned a 500 Internal Server Error. " +
			"This is a transient orchestrator-side issue; retry in a few minutes. " +
			"Original error: " + err.Error()
	}

	diags.AddError(summary, detail)
}

// pdpTagPattern extracts the missing tag name(s) from a PDP message like:
//
//	"Your project is lacking VM.NoLimitCPU or VM.NoLimit tag to provision …"
//
// Returns the substring listing the tags, or "" if no tags can be parsed.
var pdpTagPattern = regexp.MustCompile(`lacking ([A-Za-z0-9_.\- ]+?) tag`)

// pdpDiagnostic returns a tag-aware diagnostic when err's text looks like a
// FABRIC PDP policy violation. ok=false means err is unrelated.
func (r *SliceResource) pdpDiagnostic(err error) (summary, detail string, ok bool) {
	msg := err.Error()
	if !strings.Contains(msg, "PDP Failure") && !strings.Contains(msg, "Policy Violation") {
		return "", "", false
	}

	// Show only tags the token actually carries — the orchestrator is the
	// ground truth here, and listing user-asserted-but-not-granted tags
	// would mislead the user about what they hold.
	claims := r.tokenSource.Claims()
	var known []string
	if claims != nil {
		known = append(known, claims.Project().Tags...)
	}
	sort.Strings(known)
	knownStr := "(none discovered from token)"
	if len(known) > 0 {
		knownStr = strings.Join(known, ", ")
	}

	missing := ""
	if m := pdpTagPattern.FindStringSubmatch(msg); len(m) == 2 {
		missing = strings.TrimSpace(m[1])
	}

	summary = "FABRIC Policy Violation"
	if missing != "" {
		detail = fmt.Sprintf(
			"The orchestrator rejected this slice because project %q is missing the %s tag.\n\n"+
				"Tags currently available on this project (from your token): %s\n\n"+
				"Either reduce the requested capacity/components so an available tag covers it, "+
				"or ask your FABRIC project lead to add the missing tag at "+
				"https://portal.fabric-testbed.net → Projects, then request a fresh token.\n\n"+
				"Original error: %s",
			r.projectName(), missing, knownStr, msg)
	} else {
		detail = fmt.Sprintf(
			"The orchestrator rejected this slice with a policy violation.\n\n"+
				"Tags currently available on project %q (from your token): %s\n\n"+
				"Compare the requested resources against what those tags allow. "+
				"To add a tag, contact your FABRIC project lead.\n\n"+
				"Original error: %s",
			r.projectName(), knownStr, msg)
	}
	return summary, detail, true
}

func (r *SliceResource) projectName() string {
	if r.tokenSource == nil || r.tokenSource.Claims() == nil {
		return "unknown project"
	}
	project := r.tokenSource.Claims().Project()
	if project.Name != "" {
		return project.Name
	}
	if project.UUID != "" {
		return project.UUID
	}
	return "unknown project"
}
