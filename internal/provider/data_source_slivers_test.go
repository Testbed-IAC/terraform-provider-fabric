package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func sliversSchema(ctx context.Context) dschema.Schema {
	var sr datasource.SchemaResponse
	(&SliversDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &sr)
	return sr.Schema
}

func sliversConfig(t *testing.T, ctx context.Context, model SliversDataSourceModel) tfsdk.Config {
	t.Helper()
	s := sliversSchema(ctx)
	state := &tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding slivers model: %v", diags)
	}
	return tfsdk.Config{Schema: s, Raw: state.Raw}
}

func readSlivers(t *testing.T, ctx context.Context, client *fake.Client, model SliversDataSourceModel) (SliversDataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	d := &SliversDataSource{client: client}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: sliversSchema(ctx)}}
	d.Read(ctx, datasource.ReadRequest{Config: sliversConfig(t, ctx, model)}, resp)
	var got SliversDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading slivers state: %v", diags)
		}
	}
	return got, resp
}

func TestFabric_DataSource_Slivers_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("maps sliver state", func(t *testing.T) {
		t.Parallel()
		var gotSliceID string
		client := &fake.Client{SliversFn: func(_ context.Context, sliceID string) ([]fabricclient.Sliver, error) {
			gotSliceID = sliceID
			return []fabricclient.Sliver{{
				SliverID:     "sliver-1",
				SliverType:   "NodeSliver",
				State:        "Active",
				PendingState: "None",
				JoinState:    "Joined",
				ManagementIP: "192.0.2.10",
				GraphNodeID:  "graph-node-1",
			}}, nil
		}}
		got, resp := readSlivers(t, ctx, client, SliversDataSourceModel{SliceID: types.StringValue("slice-1")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotSliceID != "slice-1" {
			t.Fatalf("slice_id passed to client = %q, want slice-1", gotSliceID)
		}
		if len(got.Slivers) != 1 {
			t.Fatalf("slivers = %d, want 1", len(got.Slivers))
		}
		sliver := got.Slivers[0]
		if sliver.SliverID.ValueString() != "sliver-1" || sliver.ManagementIP.ValueString() != "192.0.2.10" {
			t.Fatalf("sliver = %+v, want id and management IP", sliver)
		}
		if sliver.State.ValueString() != "Active" || sliver.PendingState.ValueString() != "None" || sliver.JoinState.ValueString() != "Joined" {
			t.Fatalf("sliver state = %+v, want active/none/joined", sliver)
		}
	})

	t.Run("client error becomes diagnostic", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{SliversFn: func(context.Context, string) ([]fabricclient.Sliver, error) {
			return nil, errors.New("boom")
		}}
		_, resp := readSlivers(t, ctx, client, SliversDataSourceModel{SliceID: types.StringValue("slice-1")})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error diagnostic when client fails")
		}
	})
}
