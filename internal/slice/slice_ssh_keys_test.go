package slice

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	if !got.SSHKey.IsNull() || !got.SSHKeys.IsNull() {
		t.Fatalf("ssh key state = ssh_key:%v ssh_keys:%v, want both null", got.SSHKey, got.SSHKeys)
	}
}

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

func TestSSHKeyAliasDeprecationMessage(t *testing.T) {
	t.Parallel()
	attr, ok := sliceResourceSchema(context.Background()).Attributes["ssh_key"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("ssh_key schema type = %T, want schema.StringAttribute", sliceResourceSchema(context.Background()).Attributes["ssh_key"])
	}
	if !strings.Contains(attr.DeprecationMessage, "ssh_keys") {
		t.Fatalf("deprecation message = %q, want ssh_keys guidance", attr.DeprecationMessage)
	}
}
