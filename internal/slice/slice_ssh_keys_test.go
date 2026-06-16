package slice

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fake "github.com/Testbed-IAC/fabric-go-fim/pkg/client/clienttest"
)

func TestSliceResourceSSHKeysFlowToCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := bareSliceModel()
	model.SSHKey = types.StringNull()
	model.SSHKeys = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("ssh-ed25519 AAAA first"),
		types.StringValue("ssh-ed25519 BBBB second"),
	})
	_, graphML, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}

	client := &fake.Client{
		CreateFn: func(_ context.Context, _ string, _ string, keys []string, _ fabricclient.CreateOpts) ([]fabricclient.Sliver, error) {
			if len(keys) != 2 || keys[0] != "ssh-ed25519 AAAA first" || keys[1] != "ssh-ed25519 BBBB second" {
				t.Fatalf("CreateSlice keys = %#v, want two configured keys", keys)
			}
			return []fabricclient.Sliver{{SliceID: "slice-1"}}, nil
		},
		GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
			return &fabricclient.Slice{SliceID: "slice-1", Name: "slice", State: "StableOK", GraphID: "graph-1", Model: graphML}, nil
		},
	}
	r := &SliceResource{client: client}
	resp := &resource.CreateResponse{State: emptySliceState(ctx)}
	r.Create(ctx, resource.CreateRequest{Plan: slicePlan(t, ctx, model), Config: sliceConfig(t, ctx, model)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", resp.Diagnostics.Errors())
	}

	var got SliceResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	// The deprecated write-only ssh_key is stripped from state; the current ssh_keys
	// list is preserved (it is Sensitive, not write-only).
	if !got.SSHKey.IsNull() {
		t.Fatalf("ssh_key state = %v, want null (write-only)", got.SSHKey)
	}
	var gotKeys []string
	if diags := got.SSHKeys.ElementsAs(ctx, &gotKeys, false); diags.HasError() {
		t.Fatalf("reading ssh_keys: %v", diags)
	}
	want := []string{"ssh-ed25519 AAAA first", "ssh-ed25519 BBBB second"}
	if len(gotKeys) != len(want) || gotKeys[0] != want[0] || gotKeys[1] != want[1] {
		t.Fatalf("ssh_keys state = %v, want preserved %v", gotKeys, want)
	}
}

// TestSSHKeySourceValidation asserts the XOR contract: validateSSHKeySource errors
// when ssh_key and ssh_keys are both set OR both unset, and the diagnostic attaches
// to the ssh_keys path.
func TestSSHKeySourceValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		model     SliceResourceModel
		wantError bool
	}{
		{name: "deprecated single key", model: SliceResourceModel{SSHKey: types.StringValue("ssh-ed25519 AAAA")}},
		{name: "multiple keys", model: SliceResourceModel{SSHKeys: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ssh-ed25519 AAAA")})}},
		{name: "neither set", model: SliceResourceModel{}, wantError: true},
		{
			name: "both set",
			model: SliceResourceModel{
				SSHKey:  types.StringValue("ssh-ed25519 AAAA"),
				SSHKeys: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ssh-ed25519 BBBB")}),
			},
			wantError: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			validateSSHKeySource(tc.model, &diags)
			if got := diags.HasError(); got != tc.wantError {
				t.Fatalf("HasError = %t, want %t: %v", got, tc.wantError, diags.Errors())
			}
			if tc.wantError && diagnosticPath(t, diags.Errors()[0]) != "ssh_keys" {
				t.Fatalf("diagnostic path = %q, want ssh_keys", diagnosticPath(t, diags.Errors()[0]))
			}
		})
	}
}
