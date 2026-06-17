package slice

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

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/fabtime"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/poller"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// SliceResource manages a FABRIC slice: it builds a FIM topology from the
// configured blocks, submits it to the orchestrator, and polls for a terminal
// provisioning state before recording computed runtime fields.
type SliceResource struct {
	client          fabricclient.API
	tokenSource     auth.TokenSource
	resourcesSource providercfg.ResourcesSummarySource
}

// NewResource returns the FABRIC slice resource.
func NewResource() resource.Resource {
	return &SliceResource{}
}

func (r *SliceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slice"
}

func (r *SliceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = sliceResourceSchema(ctx)
}

func (r *SliceResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{sshKeySourceValidator{}}
}

func (r *SliceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := providercfg.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if data == nil {
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
	normalizeLeasePlan(&plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateLabelConfiguration(plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	validatePermissionTags(permissionRequest(plan), r.tokenSource, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := validateCatalog(ctx, plan); err != nil {
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
	validateSSHKeySource(config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	sshKeys, err := configuredSSHKeys(ctx, config)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("ssh_keys"),
			"Invalid SSH key configuration",
			"Configure exactly one of ssh_keys or the deprecated ssh_key alias. Use ssh_keys for one or more SSH public keys. Original error: "+err.Error(),
		)
		return
	}
	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		addBuildError(&resp.Diagnostics, err)
		return
	}
	tflog.Info(ctx, "creating FABRIC slice", map[string]any{"slice_name": plan.Name.ValueString(), "node_count": len(plan.Nodes), "network_count": len(plan.Networks)})
	tflog.Trace(ctx, "sending FABRIC GraphML", map[string]any{"graphml_bytes": len(graphML), "graphml_preview": preview(graphML)})
	leaseStartTime, err := tfutil.CanonicalFabricTimeString(tfutil.StringValue(plan.LeaseStartTime))
	if err != nil {
		addLeaseTimeError(&resp.Diagnostics, "lease_start_time", err)
		return
	}
	slivers, err := r.client.CreateSlice(ctx, plan.Name.ValueString(), graphML, sshKeys, fabricclient.CreateOpts{
		LifetimeHours:  int32(tfutil.Int64Value(plan.LifetimeHours)),
		LeaseStartTime: leaseStartTime,
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
	clearSSHKeys(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SetField(ctx, "slice_id", sliceID)
	timeout, diags := plan.Timeouts.Create(ctx, defaultLifecycleTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	interval := tfutil.PollInterval(slicePollInterval)
	final, err := poller.WaitForSlice(ctx, r.client, sliceID, []string{stateStableOK}, []string{stateStableError, stateAllocatedError, stateDead, stateClosingError}, timeout, interval)
	if err != nil {
		tflog.Error(ctx, "slice did not become stable", map[string]any{"err": err})
		resp.Diagnostics.AddError("FABRIC slice did not become stable", err.Error())
		return
	}
	final = r.waitForNodeManagementIPs(ctx, sliceID, final, managementIPWaitBudget, interval)
	if err := r.refreshFromSlice(ctx, final, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Refresh FABRIC slice failed", err.Error())
		return
	}
	clearSSHKeys(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SliceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SliceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sliceID := tfutil.StringValue(state.SliceID)
	if sliceID == "" {
		sliceID = tfutil.StringValue(state.ID)
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
	case stateDead:
		resp.State.RemoveResource(ctx)
		return
	case stateClosing, stateClosingError:
		tflog.Warn(ctx, "slice is closing and will be removed from state", map[string]any{"slice_id": sliceID, "state": slice.State})
		resp.Diagnostics.AddWarning("FABRIC slice is closing", "The slice is in "+slice.State+" and has been removed from Terraform state.")
		resp.State.RemoveResource(ctx)
		return
	case stateStableError, stateModifyError:
		resp.Diagnostics.AddWarning("FABRIC slice in error state", slice.Notice)
	case stateModifyOK:
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

// Update recovers a slice stuck in ModifyOK, then routes the change: a
// lifetime-only change renews the lease in place (renewLease); any topology
// change goes through the modify+accept path (modifyTopology).
func (r *SliceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SliceResourceModel
	var state SliceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sliceID := state.SliceID.ValueString()
	if cur, err := r.client.GetSlice(ctx, sliceID); err == nil && cur != nil && cur.State == stateModifyOK {
		tflog.Warn(ctx, "recovering previous ModifyOK before update", map[string]any{"slice_id": sliceID, "previous_state": cur.State})
		if _, err := r.client.AcceptModify(ctx, sliceID); err != nil {
			resp.Diagnostics.AddError("Accept previous FABRIC modify failed", err.Error())
			return
		}
	}
	if topologyEquivalent(ctx, state, plan) && state.LifetimeHours.ValueInt64() != plan.LifetimeHours.ValueInt64() {
		r.renewLease(ctx, sliceID, &plan, resp)
		return
	}
	r.modifyTopology(ctx, sliceID, &plan, resp)
}

// renewLease handles the lifetime-only update fast path: it renews the lease,
// deriving the new end from lease_end_time when set or from lifetime_hours
// otherwise, then refreshes state without rebuilding the topology.
func (r *SliceResource) renewLease(ctx context.Context, sliceID string, plan *SliceResourceModel, resp *resource.UpdateResponse) {
	leaseEnd := tfutil.StringValue(plan.LeaseEndTime)
	if leaseEnd == "" {
		leaseEnd = fabtime.Format(time.Now().UTC().Add(time.Duration(plan.LifetimeHours.ValueInt64()) * time.Hour))
	} else {
		var err error
		leaseEnd, err = tfutil.CanonicalFabricTimeString(leaseEnd)
		if err != nil {
			addLeaseTimeError(&resp.Diagnostics, "lease_end_time", err)
			return
		}
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
	if err := r.refreshFromSlice(ctx, slice, plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Refresh FABRIC slice failed", err.Error())
		return
	}
	clearSSHKeys(plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// modifyTopology handles a topology-changing update: it submits the rebuilt
// GraphML, waits for the modify to reach a terminal state, accepts it (pruning
// any partial failures as a warning), and refreshes state.
func (r *SliceResource) modifyTopology(ctx context.Context, sliceID string, plan *SliceResourceModel, resp *resource.UpdateResponse) {
	_, graphML, err := buildTopology(ctx, *plan)
	if err != nil {
		addBuildError(&resp.Diagnostics, err)
		return
	}
	tflog.Info(ctx, "updating FABRIC slice", map[string]any{"slice_id": sliceID})
	if _, err := r.client.ModifySlice(ctx, sliceID, graphML); err != nil {
		r.addClientError(&resp.Diagnostics, "Modify FABRIC slice failed", err)
		return
	}
	timeout, diags := plan.Timeouts.Update(ctx, defaultLifecycleTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	final, err := poller.WaitForSlice(ctx, r.client, sliceID, []string{stateModifyOK, stateModifyError, stateStableOK}, []string{stateDead, stateClosingError}, timeout, tfutil.PollInterval(slicePollInterval))
	if err != nil {
		resp.Diagnostics.AddError("FABRIC modify did not complete", err.Error())
		return
	}
	if final.State == stateModifyError {
		tflog.Warn(ctx, "FABRIC modify reached ModifyError; accepting to prune failures", map[string]any{"slice_id": sliceID, "notice": final.Notice})
		resp.Diagnostics.AddWarning("FABRIC modify partially failed", final.Notice)
	}
	accepted, err := r.client.AcceptModify(ctx, sliceID)
	if err != nil {
		resp.Diagnostics.AddError("Accept FABRIC modify failed", err.Error())
		return
	}
	accepted = r.waitForNodeManagementIPs(ctx, sliceID, accepted, managementIPWaitBudget, tfutil.PollInterval(slicePollInterval))
	if err := r.refreshFromSlice(ctx, accepted, plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Refresh FABRIC slice failed", err.Error())
		return
	}
	clearSSHKeys(plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
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
	timeout, diags := state.Timeouts.Delete(ctx, defaultLifecycleTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := poller.WaitForSlice(ctx, r.client, sliceID, []string{stateDead, stateClosingError}, nil, timeout, tfutil.PollInterval(slicePollInterval)); err != nil {
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
	leaseStart, err := canonicalLeaseField(slice.LeaseStartTime, state.LeaseStartTime, "lease start time")
	if err != nil {
		return err
	}
	state.LeaseStartTime = leaseStart
	leaseEnd, err := canonicalLeaseField(slice.LeaseEndTime, state.LeaseEndTime, "lease end time")
	if err != nil {
		return err
	}
	state.LeaseEndTime = leaseEnd
	if slice.Model != "" {
		actual, err := topology.Load(strings.NewReader(slice.Model))
		if err != nil {
			return fmt.Errorf("loading returned topology: %w", err)
		}
		desired, _, err := buildTopology(ctx, *state)
		if err != nil {
			return fmt.Errorf("building desired topology for drift comparison: %w", err)
		}
		diff := topology.DiffTopologies(desired, actual)
		if diff.HasUserIntentChanges() {
			tflog.Info(ctx, "FABRIC topology drift detected", map[string]any{"slice_id": slice.SliceID, "diff_summary": diff.Summary()})
			for _, d := range diff.UserIntentDiagnostics() {
				diags.AddAttributeWarning(
					driftDiagnosticPath(d.Field()),
					"FABRIC topology drift detected",
					d.Error()+"\n\n"+d.Suggestion()+" FABRIC allocation fields are treated as computed state; configuration-owned drift must be reconciled by updating configuration or replacing/modifying the slice.",
				)
			}
		}
		if err := r.setNodeOutputs(ctx, slice.SliceID, actual, state); err != nil {
			return err
		}
	}
	return nil
}

func driftDiagnosticPath(field string) path.Path {
	if strings.Contains(field, ".edges.") {
		return path.Root("network")
	}
	if strings.Contains(field, ".nodes.") {
		return path.Root("node")
	}
	return path.Root("id")
}

// waitForNodeManagementIPs re-polls the slice ASM after it has reached a stable
// state until every VM NetworkNode carries a management IP, or budget elapses.
// The AM assigns the management IP as the reservation finishes activating, and
// the orchestrator can briefly report a stable slice state before that IP is
// reflected in the ASM; without this, setNodeOutputs records an empty
// management_ip. Best-effort: if no IP appears within budget the latest slice is
// returned and the caller proceeds with the empty value, preserving prior
// behavior. Returns immediately (no extra fetch) when current already has IPs or
// has no VM nodes to wait on.
func (r *SliceResource) waitForNodeManagementIPs(ctx context.Context, sliceID string, current *fabricclient.Slice, budget, interval time.Duration) *fabricclient.Slice {
	if allVMNodesHaveMgmtIP(current) {
		return current
	}
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return current
		case <-time.After(interval):
		}
		slice, err := r.client.GetSlice(ctx, sliceID)
		if err != nil || slice == nil {
			tflog.Warn(ctx, "polling slice for management IPs failed", map[string]any{"slice_id": sliceID, "err": err})
			continue
		}
		current = slice
		if allVMNodesHaveMgmtIP(current) {
			return current
		}
	}
	tflog.Warn(ctx, "management IPs not populated before deadline; proceeding", map[string]any{"slice_id": sliceID})
	return current
}

// allVMNodesHaveMgmtIP reports whether every VM NetworkNode in the slice ASM has
// a non-empty management IP. A slice with no VM nodes (e.g. network-only) is
// considered ready, as is one whose model fails to load — there is nothing to
// wait for in either case.
func allVMNodesHaveMgmtIP(slice *fabricclient.Slice) bool {
	if slice == nil || slice.Model == "" {
		return false
	}
	topo, err := topology.Load(strings.NewReader(slice.Model))
	if err != nil {
		return true
	}
	for _, node := range topo.Nodes() {
		s, err := node.Sliver()
		if err != nil {
			return true
		}
		if s.Type != sliver.NodeTypeVM {
			continue
		}
		if s.MgmtIP == "" {
			return false
		}
	}
	return true
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

func normalizeLeasePlan(plan *SliceResourceModel, diags *diag.Diagnostics) {
	leaseStartTime, err := tfutil.CanonicalFabricTimeValue(plan.LeaseStartTime)
	if err != nil {
		addLeaseTimeError(diags, "lease_start_time", err)
		return
	}
	plan.LeaseStartTime = leaseStartTime
	leaseEndTime, err := tfutil.CanonicalFabricTimeValue(plan.LeaseEndTime)
	if err != nil {
		addLeaseTimeError(diags, "lease_end_time", err)
		return
	}
	plan.LeaseEndTime = leaseEndTime
}

// addLeaseTimeError records the standard invalid-lease-time diagnostic for the
// lease_start_time/lease_end_time attributes. leaseField is the schema attribute
// name (e.g. "lease_start_time"); the wording is kept identical to
// tfutil.FabricTimeValidator so a value rejected here reads the same as one
// rejected at plan-validation time.
func addLeaseTimeError(diags *diag.Diagnostics, leaseField string, err error) {
	human := strings.ReplaceAll(strings.TrimSuffix(leaseField, "_time"), "_", " ")
	diags.AddAttributeError(
		path.Root(leaseField),
		"Invalid FABRIC "+human+" time",
		"The "+leaseField+" value must use the FABRIC orchestrator format ("+tfutil.FabricTimeExamples+") or RFC3339. Original error: "+err.Error(),
	)
}

// canonicalLeaseField returns the canonical FABRIC representation of a lease
// timestamp: the orchestrator-returned sliceValue when present, otherwise the
// value already in state (re-canonicalized so a previously stored value keeps a
// stable layout). label names the bound for error context.
func canonicalLeaseField(sliceValue string, stateValue types.String, label string) (types.String, error) {
	if sliceValue != "" {
		canonical, err := tfutil.CanonicalFabricTimeString(sliceValue)
		if err != nil {
			return types.String{}, fmt.Errorf("reading %s: %w", label, err)
		}
		return types.StringValue(canonical), nil
	}
	canonical, err := tfutil.CanonicalFabricTimeValue(stateValue)
	if err != nil {
		return types.String{}, fmt.Errorf("reading %s from state: %w", label, err)
	}
	return canonical, nil
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
