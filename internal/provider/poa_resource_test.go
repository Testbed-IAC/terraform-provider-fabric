package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fake "github.com/Testbed-IAC/fabric-go-fim/pkg/client/clienttest"
	poller "github.com/Testbed-IAC/fabric-go-fim/pkg/poller"
)

func poaSchema(ctx context.Context) rschema.Schema {
	var sr resource.SchemaResponse
	(&POAResource{}).Schema(ctx, resource.SchemaRequest{}, &sr)
	return sr.Schema
}

func poaRaw(t *testing.T, ctx context.Context, model POAResourceModel) tfsdk.Plan {
	t.Helper()
	if model.Triggers.IsNull() || model.Triggers.IsUnknown() {
		model.Triggers = types.MapNull(types.StringType)
	}
	if model.Timeouts.IsNull() || model.Timeouts.IsUnknown() {
		model.Timeouts = timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
			"create": types.StringType,
			"read":   types.StringType,
		})}
	}
	s := poaSchema(ctx)
	state := &tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding poa model: %v", diags)
	}
	return tfsdk.Plan{Schema: s, Raw: state.Raw}
}

func poaState(t *testing.T, ctx context.Context, model POAResourceModel) tfsdk.State {
	t.Helper()
	plan := poaRaw(t, ctx, model)
	return tfsdk.State(plan)
}

func TestFabric_ResourcePOA_RequestMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		model POAResourceModel
		check func(t *testing.T, got fabricclient.POARequest)
	}{
		{
			name: "reboot has operation only",
			model: POAResourceModel{
				Operation: types.StringValue(poaOperationReboot),
			},
			check: func(t *testing.T, got fabricclient.POARequest) {
				t.Helper()
				if got.Operation != poaOperationReboot || len(got.NodeSet) != 0 || len(got.Keys) != 0 {
					t.Fatalf("request = %+v, want reboot only", got)
				}
			},
		},
		{
			name: "cpupin maps pinning data",
			model: POAResourceModel{
				Operation:  types.StringValue(poaOperationCPUPin),
				VCPUCPUMap: []POAVCPUCPUModel{{VCPU: types.StringValue("0"), CPU: types.StringValue("2")}},
				NodeSet:    []types.String{types.StringValue("node-1")},
			},
			check: func(t *testing.T, got fabricclient.POARequest) {
				t.Helper()
				if got.Operation != poaOperationCPUPin || len(got.VCPUCPUMap) != 1 || got.VCPUCPUMap[0].CPU != "2" || got.NodeSet[0] != "node-1" {
					t.Fatalf("request = %+v, want cpupin data", got)
				}
			},
		},
		{
			name: "addkey maps keys",
			model: POAResourceModel{
				Operation: types.StringValue(poaOperationAddKey),
				Keys:      []POAKeyModel{{Key: types.StringValue("ssh-ed25519 AAAA"), Comment: types.StringValue("test")}},
			},
			check: func(t *testing.T, got fabricclient.POARequest) {
				t.Helper()
				if got.Operation != poaOperationAddKey || len(got.Keys) != 1 || got.Keys[0].Comment != "test" {
					t.Fatalf("request = %+v, want key data", got)
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, poaRequestFromModel(tc.model))
		})
	}
}

func TestFabric_ResourcePOA_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var gotSliverID string
	var gotRequest fabricclient.POARequest
	client := &fake.Client{
		CreatePOAFn: func(_ context.Context, sliverID string, request fabricclient.POARequest) (*fabricclient.POA, error) {
			gotSliverID = sliverID
			gotRequest = request
			return &fabricclient.POA{POAID: "poa-1", Operation: request.Operation, State: "Running"}, nil
		},
		GetPOAFn: func(context.Context, string) (*fabricclient.POA, error) {
			return &fabricclient.POA{POAID: "poa-1", Operation: poaOperationReboot, State: poller.POASuccessState, InfoJSON: `{"result":"ok"}`}, nil
		},
	}
	r := &POAResource{client: client}
	model := POAResourceModel{
		SliverID:  types.StringValue("sliver-1"),
		Operation: types.StringValue(poaOperationReboot),
	}
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: poaSchema(ctx)}}
	r.Create(ctx, resource.CreateRequest{Plan: poaRaw(t, ctx, model)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", resp.Diagnostics.Errors())
	}
	if gotSliverID != "sliver-1" || gotRequest.Operation != poaOperationReboot {
		t.Fatalf("CreatePOA args = %q/%+v, want sliver-1/reboot", gotSliverID, gotRequest)
	}
	var got POAResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if got.POAID.ValueString() != "poa-1" || got.State.ValueString() != poller.POASuccessState || got.Info.ValueString() != `{"result":"ok"}` {
		t.Fatalf("state = %+v, want terminal POA state", got)
	}
}

func TestFabric_ResourcePOA_CreateFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &fake.Client{
		CreatePOAFn: func(_ context.Context, _ string, request fabricclient.POARequest) (*fabricclient.POA, error) {
			return &fabricclient.POA{POAID: "poa-1", Operation: request.Operation, State: "Running"}, nil
		},
		GetPOAFn: func(context.Context, string) (*fabricclient.POA, error) {
			return &fabricclient.POA{POAID: "poa-1", Operation: poaOperationReboot, State: poller.POAFailedState, Error: "reboot failed"}, nil
		},
	}
	r := &POAResource{client: client}
	model := POAResourceModel{
		SliverID:  types.StringValue("sliver-1"),
		Operation: types.StringValue(poaOperationReboot),
	}
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: poaSchema(ctx)}}
	r.Create(ctx, resource.CreateRequest{Plan: poaRaw(t, ctx, model)}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics when POA reaches Failed")
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "reboot failed") {
		t.Fatalf("diagnostic detail = %q, want POA error", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestFabric_ResourcePOA_ReadFailedWarns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &fake.Client{GetPOAFn: func(context.Context, string) (*fabricclient.POA, error) {
		return &fabricclient.POA{POAID: "poa-1", State: poller.POAFailedState, Error: "operation failed"}, nil
	}}
	r := &POAResource{client: client}
	state := POAResourceModel{
		ID:        types.StringValue("poa-1"),
		POAID:     types.StringValue("poa-1"),
		SliverID:  types.StringValue("sliver-1"),
		Operation: types.StringValue(poaOperationReboot),
	}
	resp := &resource.ReadResponse{State: tfsdk.State{Schema: poaSchema(ctx)}}
	r.Read(ctx, resource.ReadRequest{State: poaState(t, ctx, state)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
	}
	if len(resp.Diagnostics.Warnings()) != 1 {
		t.Fatalf("warnings = %d, want 1", len(resp.Diagnostics.Warnings()))
	}
}
