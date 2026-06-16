package slice

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fake "github.com/Testbed-IAC/fabric-go-fim/pkg/client/clienttest"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/fabtime"
)

// sliceRaw serializes a SliceResourceModel into the tftypes value that backs a
// plan/config/state for the slice resource schema. It lets the lifecycle tests
// drive the real Create/Read/Update/Delete methods rather than asserting on a
// fake client's call log.
func sliceRaw(t *testing.T, ctx context.Context, model SliceResourceModel) tftypes.Value {
	t.Helper()
	model = cloneSliceResourceModelForTest(model)
	// The framework requires fully-typed null values for the computed map and
	// timeouts block; their zero values carry no element/attribute types.
	if model.NodeOutputs.IsNull() || model.NodeOutputs.IsUnknown() {
		model.NodeOutputs = types.MapNull(types.ObjectType{AttrTypes: nodeOutputAttrTypes()})
	}
	if model.SSHKeys.IsNull() || model.SSHKeys.IsUnknown() {
		model.SSHKeys = types.ListNull(types.StringType)
	}
	for i := range model.Nodes {
		if model.Nodes[i].PostBootExecute.IsNull() || model.Nodes[i].PostBootExecute.IsUnknown() {
			model.Nodes[i].PostBootExecute = types.ListNull(types.StringType)
		}
		if model.Nodes[i].PostUpdate.IsNull() || model.Nodes[i].PostUpdate.IsUnknown() {
			model.Nodes[i].PostUpdate = types.ListNull(types.StringType)
		}
	}
	if model.Timeouts.IsNull() || model.Timeouts.IsUnknown() {
		model.Timeouts = timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
			"create": types.StringType,
			"read":   types.StringType,
			"update": types.StringType,
			"delete": types.StringType,
		})}
	}
	state := &tfsdk.State{Schema: sliceResourceSchema(ctx)}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding slice model: %v", diags)
	}
	return state.Raw
}

func cloneSliceResourceModelForTest(model SliceResourceModel) SliceResourceModel {
	model.Nodes = append([]NodeModel(nil), model.Nodes...)
	for i := range model.Nodes {
		model.Nodes[i].Components = append([]ComponentModel(nil), model.Nodes[i].Components...)
		model.Nodes[i].Storage = append([]StorageModel(nil), model.Nodes[i].Storage...)
		model.Nodes[i].Routes = append([]RouteModel(nil), model.Nodes[i].Routes...)
		model.Nodes[i].PostBootUploads = append([]PostBootUploadModel(nil), model.Nodes[i].PostBootUploads...)
	}

	model.Networks = append([]NetworkModel(nil), model.Networks...)
	for i := range model.Networks {
		model.Networks[i].Interfaces = append([]InterfaceModel(nil), model.Networks[i].Interfaces...)
		for j := range model.Networks[i].Interfaces {
			model.Networks[i].Interfaces[j].SubInterfaces = append([]SubInterfaceModel(nil), model.Networks[i].Interfaces[j].SubInterfaces...)
		}
		if model.Networks[i].Gateway != nil {
			gateway := *model.Networks[i].Gateway
			model.Networks[i].Gateway = &gateway
		}
	}

	model.Facilities = append([]FacilityPortModel(nil), model.Facilities...)
	for i := range model.Facilities {
		model.Facilities[i].Interfaces = append([]FacilityPortInterfaceModel(nil), model.Facilities[i].Interfaces...)
	}

	model.Switches = append([]SwitchModel(nil), model.Switches...)
	return model
}

func slicePlan(t *testing.T, ctx context.Context, model SliceResourceModel) tfsdk.Plan {
	return tfsdk.Plan{Schema: sliceResourceSchema(ctx), Raw: sliceRaw(t, ctx, model)}
}

func sliceConfig(t *testing.T, ctx context.Context, model SliceResourceModel) tfsdk.Config {
	return tfsdk.Config{Schema: sliceResourceSchema(ctx), Raw: sliceRaw(t, ctx, model)}
}

func sliceState(t *testing.T, ctx context.Context, model SliceResourceModel) tfsdk.State {
	return tfsdk.State{Schema: sliceResourceSchema(ctx), Raw: sliceRaw(t, ctx, model)}
}

func emptySliceState(ctx context.Context) tfsdk.State {
	return tfsdk.State{Schema: sliceResourceSchema(ctx)}
}

// nodeGraphID returns the deterministic graph node id the topology builder
// assigns to a named node, so tests can correlate slivers by graph node id.
func nodeGraphID(t *testing.T, ctx context.Context, model SliceResourceModel, name string) string {
	t.Helper()
	topo, _, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	node, ok := topo.Node(name)
	if !ok {
		t.Fatalf("node %q not found in built topology", name)
	}
	return node.ID()
}

func bareSliceModel() SliceResourceModel {
	m := bareVMModel()
	m.Name = types.StringValue("slice")
	m.SSHKey = types.StringValue("ssh-ed25519 AAAA")
	m.LifetimeHours = types.Int64Value(24)
	return m
}

func TestFabric_ResourceSlice_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	model.LeaseStartTime = types.StringValue("2026-05-30T19:04:54Z")
	_, graphML, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	nodeID := nodeGraphID(t, ctx, model, "vm1")

	client := &fake.Client{
		CreateFn: func(_ context.Context, name, gml string, keys []string, opts fabricclient.CreateOpts) ([]fabricclient.Sliver, error) {
			if name != "slice" {
				t.Errorf("CreateSlice name = %q, want slice", name)
			}
			if len(keys) != 1 || keys[0] != "ssh-ed25519 AAAA" {
				t.Errorf("CreateSlice ssh keys = %#v, want one configured key", keys)
			}
			if opts.LifetimeHours != 24 {
				t.Errorf("CreateSlice lifetime = %d, want 24", opts.LifetimeHours)
			}
			if opts.LeaseStartTime != "2026-05-30 19:04:54 +00:00" {
				t.Errorf("CreateSlice lease_start_time = %q, want FABRIC layout", opts.LeaseStartTime)
			}
			return []fabricclient.Sliver{{SliceID: "slice-1"}}, nil
		},
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
		SliversFn: func(context.Context, string) ([]fabricclient.Sliver, error) {
			return []fabricclient.Sliver{{SliverID: "sliver-1", GraphNodeID: nodeID, State: "Active"}}, nil
		},
	}
	r := &SliceResource{client: client}

	resp := &resource.CreateResponse{State: emptySliceState(ctx)}
	r.Create(ctx, resource.CreateRequest{
		Plan:   slicePlan(t, ctx, model),
		Config: sliceConfig(t, ctx, model),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", resp.Diagnostics.Errors())
	}

	var got SliceResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if got.SliceID.ValueString() != "slice-1" {
		t.Errorf("slice_id = %q, want slice-1", got.SliceID.ValueString())
	}
	if got.State.ValueString() != "StableOK" {
		t.Errorf("state = %q, want StableOK", got.State.ValueString())
	}
	if !got.SSHKey.IsNull() {
		t.Errorf("ssh_key = %v, want null after create (write-only must not persist)", got.SSHKey)
	}
	if !got.SSHKeys.IsNull() {
		t.Errorf("ssh_keys = %v, want null after create (sensitive keys must not persist)", got.SSHKeys)
	}
	outputs := map[string]NodeOutputModel{}
	if diags := got.NodeOutputs.ElementsAs(ctx, &outputs, false); diags.HasError() {
		t.Fatalf("decoding node outputs: %v", diags)
	}
	vm1, ok := outputs["vm1"]
	if !ok {
		t.Fatalf("node outputs missing vm1: %#v", outputs)
	}
	if vm1.SliverID.ValueString() != "sliver-1" {
		t.Errorf("vm1 sliver_id = %q, want sliver-1", vm1.SliverID.ValueString())
	}
	if vm1.State.ValueString() != "Active" {
		t.Errorf("vm1 state = %q, want Active", vm1.State.ValueString())
	}
	if vm1.GraphNodeID.ValueString() != nodeID {
		t.Errorf("vm1 graph_node_id = %q, want %q", vm1.GraphNodeID.ValueString(), nodeID)
	}
}

func TestFabric_ResourceSlice_CreateNoSliversError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	client := &fake.Client{
		CreateFn: func(context.Context, string, string, []string, fabricclient.CreateOpts) ([]fabricclient.Sliver, error) {
			return nil, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.CreateResponse{State: emptySliceState(ctx)}
	r.Create(ctx, resource.CreateRequest{Plan: slicePlan(t, ctx, model), Config: sliceConfig(t, ctx, model)}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when orchestrator returns no slivers")
	}
}

func TestFabric_ResourceSlice_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	model.SliceID = types.StringValue("slice-1")
	model.ID = types.StringValue("slice-1")
	_, graphML, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	nodeID := nodeGraphID(t, ctx, model, "vm1")

	cases := []struct {
		name        string
		getFn       func(context.Context, string) (*fabricclient.Slice, error)
		wantRemoved bool
		wantState   string
	}{
		{
			name: "stable ok refreshes state",
			getFn: func(context.Context, string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
			},
			wantState: "StableOK",
		},
		{
			name: "dead removes resource",
			getFn: func(context.Context, string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: "slice-1", State: "Dead"}, nil
			},
			wantRemoved: true,
		},
		{
			name: "not found removes resource",
			getFn: func(context.Context, string) (*fabricclient.Slice, error) {
				return nil, fabricclient.ErrNotFound
			},
			wantRemoved: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{
				GetFn: tc.getFn,
				SliversFn: func(context.Context, string) ([]fabricclient.Sliver, error) {
					return []fabricclient.Sliver{{SliverID: "sliver-1", GraphNodeID: nodeID, State: "Active"}}, nil
				},
			}
			r := &SliceResource{client: client}
			resp := &resource.ReadResponse{State: sliceState(t, ctx, model)}
			r.Read(ctx, resource.ReadRequest{State: sliceState(t, ctx, model)}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
			}
			if tc.wantRemoved {
				if !resp.State.Raw.IsNull() {
					t.Fatalf("expected resource removed from state, got %v", resp.State.Raw)
				}
				return
			}
			var got SliceResourceModel
			if diags := resp.State.Get(ctx, &got); diags.HasError() {
				t.Fatalf("reading state: %v", diags)
			}
			if got.State.ValueString() != tc.wantState {
				t.Fatalf("state = %q, want %q", got.State.ValueString(), tc.wantState)
			}
			if tc.name == "stable ok refreshes state" && len(resp.Diagnostics.Warnings()) != 0 {
				t.Fatalf("warnings = %v, want none for unchanged topology", resp.Diagnostics.Warnings())
			}
		})
	}
}

func TestFabric_ResourceSlice_ReadDriftWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	model.SliceID = types.StringValue("slice-1")
	model.ID = types.StringValue("slice-1")
	_, graphML, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	actualGraphML := strings.Replace(graphML, "RENC", "UKY", 1)
	nodeID := nodeGraphID(t, ctx, model, "vm1")
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: actualGraphML}, nil
		},
		SliversFn: func(context.Context, string) ([]fabricclient.Sliver, error) {
			return []fabricclient.Sliver{{SliverID: "sliver-1", GraphNodeID: nodeID, State: "Active"}}, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.ReadResponse{State: sliceState(t, ctx, model)}

	r.Read(ctx, resource.ReadRequest{State: sliceState(t, ctx, model)}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
	}
	if len(resp.Diagnostics.Warnings()) == 0 {
		t.Fatal("expected topology drift warning")
	}
	if !strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), "configuration-owned drift") {
		t.Fatalf("warning detail = %q, want reconciliation guidance", resp.Diagnostics.Warnings()[0].Detail())
	}
}

func TestFabric_ResourceSlice_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	model.SliceID = types.StringValue("slice-1")

	t.Run("waits for dead", func(t *testing.T) {
		t.Parallel()
		deleted := false
		client := &fake.Client{
			DeleteFn: func(_ context.Context, id string) error {
				if id != "slice-1" {
					t.Errorf("DeleteSlice id = %q, want slice-1", id)
				}
				deleted = true
				return nil
			},
			GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: "slice-1", State: "Dead"}, nil
			},
		}
		r := &SliceResource{client: client}
		resp := &resource.DeleteResponse{State: sliceState(t, ctx, model)}
		r.Delete(ctx, resource.DeleteRequest{State: sliceState(t, ctx, model)}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Delete diagnostics: %v", resp.Diagnostics.Errors())
		}
		if !deleted {
			t.Fatal("DeleteSlice was not called")
		}
	})

	t.Run("already gone is success", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{
			DeleteFn: func(context.Context, string) error { return fabricclient.ErrNotFound },
		}
		r := &SliceResource{client: client}
		resp := &resource.DeleteResponse{State: sliceState(t, ctx, model)}
		r.Delete(ctx, resource.DeleteRequest{State: sliceState(t, ctx, model)}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Delete diagnostics: %v", resp.Diagnostics.Errors())
		}
	})
}

func TestFabric_ResourceSlice_UpdateRenewsWithoutModify(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")
	state.LifetimeHours = types.Int64Value(24)

	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.LifetimeHours = types.Int64Value(48) // only the lease changes

	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
		RenewFn: func(_ context.Context, _ string, leaseEndTime string) error {
			if _, err := fabtime.Parse(leaseEndTime); err != nil {
				t.Errorf("RenewSlice lease_end_time parse failed: %v", err)
			}
			if !strings.HasSuffix(leaseEndTime, "+00:00") {
				t.Errorf("RenewSlice lease_end_time = %q, want FABRIC UTC layout", leaseEndTime)
			}
			return nil
		},
		ModifyFn: func(context.Context, string, string) ([]fabricclient.Sliver, error) {
			t.Fatal("ModifySlice must not be called for a lease-only change")
			return nil, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", resp.Diagnostics.Errors())
	}
	if !containsCall(client.Calls, "RenewSlice:slice-1") {
		t.Fatalf("expected RenewSlice call, got %#v", client.Calls)
	}
}

func TestFabric_ResourceSlice_UpdateModifiesTopology(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")

	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.Nodes[0].Disk = types.Int64Value(50) // topology change forces a modify

	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}

	// First GetSlice (pre-update recovery check) returns StableOK; the poller
	// after ModifySlice sees ModifyOK.
	getStates := []string{"StableOK", "ModifyOK"}
	getCall := 0
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			s := getStates[min(getCall, len(getStates)-1)]
			getCall++
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: s, GraphID: "graph-1", Model: graphML}, nil
		},
		ModifyFn: func(context.Context, string, string) ([]fabricclient.Sliver, error) { return nil, nil },
		AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", resp.Diagnostics.Errors())
	}
	if !containsCall(client.Calls, "ModifySlice:slice-1") {
		t.Fatalf("expected ModifySlice call, got %#v", client.Calls)
	}
	if !containsCall(client.Calls, "AcceptModify:slice-1") {
		t.Fatalf("expected AcceptModify call, got %#v", client.Calls)
	}
	if containsCall(client.Calls, "RenewSlice:slice-1") {
		t.Fatalf("RenewSlice must not be called for a topology change, got %#v", client.Calls)
	}
}

func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// TestFabric_ResourceSlice_UpdateRecoversModifyOK proves the pre-update recovery
// branch: when the live slice is stuck in ModifyOK, Update accepts it before
// applying the new change (here a lease-only renew).
func TestFabric_ResourceSlice_UpdateRecoversModifyOK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")
	state.LifetimeHours = types.Int64Value(24)

	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.LifetimeHours = types.Int64Value(48) // lease-only change -> renew after recovery

	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	accepted := false
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "ModifyOK", GraphID: "graph-1", Model: graphML}, nil
		},
		AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) {
			accepted = true
			return &fabricclient.Slice{SliceID: "slice-1", State: "StableOK"}, nil
		},
		RenewFn: func(context.Context, string, string) error { return nil },
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", resp.Diagnostics.Errors())
	}
	if !accepted {
		t.Fatal("expected AcceptModify during pre-update ModifyOK recovery")
	}
	if !containsCall(client.Calls, "RenewSlice:slice-1") {
		t.Fatalf("expected renew after recovery, got %#v", client.Calls)
	}
}

// TestFabric_ResourceSlice_UpdateRecoveryAcceptFails proves Update surfaces an
// error (and does not proceed) when the pre-update ModifyOK recovery's AcceptModify
// fails.
func TestFabric_ResourceSlice_UpdateRecoveryAcceptFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")
	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.LifetimeHours = types.Int64Value(48)

	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", State: "ModifyOK"}, nil
		},
		AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return nil, errors.New("accept boom")
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when pre-update AcceptModify fails")
	}
	if containsCall(client.Calls, "RenewSlice:slice-1") {
		t.Fatalf("must not renew after a failed recovery, got %#v", client.Calls)
	}
}

// TestFabric_ResourceSlice_UpdateModifyError proves the topology-modify path warns
// (but still accepts and succeeds) when the modify settles in ModifyError, pruning
// the partial failure rather than failing the apply.
func TestFabric_ResourceSlice_UpdateModifyError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")
	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.Nodes[0].Disk = types.Int64Value(50) // topology change -> modify path

	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	// Pre-recovery GetSlice is StableOK (no recovery); the poller then sees ModifyError.
	getStates := []string{"StableOK", "ModifyError"}
	getCall := 0
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			s := getStates[min(getCall, len(getStates)-1)]
			getCall++
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: s, GraphID: "graph-1", Notice: "node vm1 failed", Model: graphML}, nil
		},
		ModifyFn: func(context.Context, string, string) ([]fabricclient.Sliver, error) { return nil, nil },
		AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", resp.Diagnostics.Errors())
	}
	if len(resp.Diagnostics.Warnings()) == 0 {
		t.Fatal("expected a ModifyError partial-failure warning")
	}
	if !containsCall(client.Calls, "AcceptModify:slice-1") {
		t.Fatalf("expected AcceptModify after ModifyError, got %#v", client.Calls)
	}
}

// TestFabric_ResourceSlice_UpdateEqualFallsToModify proves that an update whose
// topology is unchanged AND whose lifetime is unchanged takes the modify path (not
// the lease-renew fast path, which requires a lifetime change).
func TestFabric_ResourceSlice_UpdateEqualFallsToModify(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")
	state.LifetimeHours = types.Int64Value(24)
	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.LifetimeHours = types.Int64Value(24) // identical to state

	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
		ModifyFn: func(context.Context, string, string) ([]fabricclient.Sliver, error) { return nil, nil },
		AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", resp.Diagnostics.Errors())
	}
	if !containsCall(client.Calls, "ModifySlice:slice-1") {
		t.Fatalf("expected modify path for equal topology+lifetime, got %#v", client.Calls)
	}
	if containsCall(client.Calls, "RenewSlice:slice-1") {
		t.Fatalf("renew must not run without a lifetime change, got %#v", client.Calls)
	}
}

// TestFabric_ResourceSlice_UpdateRenewUsesConfiguredLeaseEnd proves the renew fast
// path canonicalizes an explicitly configured lease_end_time (RFC3339 in, FABRIC
// layout out) rather than deriving the end from lifetime_hours.
func TestFabric_ResourceSlice_UpdateRenewUsesConfiguredLeaseEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := bareSliceModel()
	state.SliceID = types.StringValue("slice-1")
	state.ID = types.StringValue("slice-1")
	state.LifetimeHours = types.Int64Value(24)
	plan := bareSliceModel()
	plan.SliceID = types.StringValue("slice-1")
	plan.ID = types.StringValue("slice-1")
	plan.LifetimeHours = types.Int64Value(48)
	plan.LeaseEndTime = types.StringValue("2026-06-01T00:00:00Z")

	_, graphML, err := buildTopology(ctx, plan)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	var gotLeaseEnd string
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
		RenewFn: func(_ context.Context, _ string, leaseEndTime string) error {
			gotLeaseEnd = leaseEndTime
			return nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.UpdateResponse{State: sliceState(t, ctx, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: slicePlan(t, ctx, plan), State: sliceState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", resp.Diagnostics.Errors())
	}
	if gotLeaseEnd != "2026-06-01 00:00:00 +00:00" {
		t.Fatalf("RenewSlice lease_end_time = %q, want canonical FABRIC layout", gotLeaseEnd)
	}
}

// TestFabric_ResourceSlice_ReadRecoversModifyOK proves Read auto-accepts a slice
// found stuck in ModifyOK and refreshes state from the accepted (StableOK) slice.
func TestFabric_ResourceSlice_ReadRecoversModifyOK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	model.SliceID = types.StringValue("slice-1")
	model.ID = types.StringValue("slice-1")
	_, graphML, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	nodeID := nodeGraphID(t, ctx, model, "vm1")
	accepted := false
	client := &fake.Client{
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "ModifyOK", GraphID: "graph-1", Model: graphML}, nil
		},
		AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) {
			accepted = true
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
		SliversFn: func(context.Context, string) ([]fabricclient.Sliver, error) {
			return []fabricclient.Sliver{{SliverID: "sliver-1", GraphNodeID: nodeID, State: "Active"}}, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.ReadResponse{State: sliceState(t, ctx, model)}
	r.Read(ctx, resource.ReadRequest{State: sliceState(t, ctx, model)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
	}
	if !accepted {
		t.Fatal("expected AcceptModify during Read ModifyOK recovery")
	}
	var got SliceResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if got.State.ValueString() != "StableOK" {
		t.Fatalf("state = %q, want StableOK after Read auto-accept", got.State.ValueString())
	}
}

// TestDriftDiagnosticPath proves drift findings are routed to the attribute path
// matching the changed field: edge diffs to the network block, node diffs to the
// node block, and anything else to the resource id.
func TestDriftDiagnosticPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		field string
		want  string
	}{
		{"topology.edges.net1.bandwidth", path.Root("network").String()},
		{"topology.nodes.vm1.site", path.Root("node").String()},
		{"topology.lease_end", path.Root("id").String()},
	}
	for _, tc := range cases {
		if got := driftDiagnosticPath(tc.field).String(); got != tc.want {
			t.Fatalf("driftDiagnosticPath(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
}
