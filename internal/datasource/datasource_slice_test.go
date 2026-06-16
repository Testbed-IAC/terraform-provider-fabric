package datasource

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fake "github.com/Testbed-IAC/fabric-go-fim/pkg/client/clienttest"
)

func sliceDataSourceSchema(ctx context.Context) dschema.Schema {
	var sr datasource.SchemaResponse
	(&SliceDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &sr)
	return sr.Schema
}

func TestFabric_DataSource_Slice_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// GetFn and ListFn return distinguishable names so the test can prove which
	// lookup path the data source actually took.
	newClient := func() *fake.Client {
		return &fake.Client{
			GetFn: func(_ context.Context, id string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: id, Name: "by-id", State: "StableOK", GraphID: "g"}, nil
			},
			ListFn: func(_ context.Context, name string, _ []string) ([]fabricclient.Slice, error) {
				return []fabricclient.Slice{{SliceID: "slice-by-name", Name: name, State: "StableOK"}}, nil
			},
		}
	}

	t.Run("by id", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		got, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{SliceID: types.StringValue("slice-1")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if !containsCall(client.Calls, "GetSlice:slice-1") {
			t.Fatalf("expected GetSlice:slice-1, got %#v", client.Calls)
		}
		if got.Name.ValueString() != "by-id" {
			t.Fatalf("name = %q, want by-id", got.Name.ValueString())
		}
	})

	t.Run("by name", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		got, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{Name: types.StringValue("my-slice")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if !containsCall(client.Calls, "ListSlices:my-slice") {
			t.Fatalf("expected ListSlices:my-slice, got %#v", client.Calls)
		}
		if got.SliceID.ValueString() != "slice-by-name" {
			t.Fatalf("slice_id = %q, want slice-by-name", got.SliceID.ValueString())
		}
	})

	t.Run("slice_id takes precedence over name", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		got, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{
			SliceID: types.StringValue("slice-1"),
			Name:    types.StringValue("my-slice"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if containsCall(client.Calls, "ListSlices:my-slice") {
			t.Fatalf("ListSlices must not be called when slice_id is set, got %#v", client.Calls)
		}
		if got.Name.ValueString() != "by-id" {
			t.Fatalf("name = %q, want by-id (lookup must use slice_id)", got.Name.ValueString())
		}
	})

	t.Run("missing lookup key errors", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		_, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error when neither slice_id nor name is set")
		}
	})

	t.Run("name not found errors", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.ListFn = func(context.Context, string, []string) ([]fabricclient.Slice, error) {
			return nil, nil
		}
		_, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{Name: types.StringValue("ghost")})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error when no slice matches the name")
		}
	})

	t.Run("by id attribute", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		got, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{ID: types.StringValue("id-1")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if !containsCall(client.Calls, "GetSlice:id-1") {
			t.Fatalf("expected GetSlice:id-1 (id fallback), got %#v", client.Calls)
		}
		if got.Name.ValueString() != "by-id" {
			t.Fatalf("name = %q, want by-id", got.Name.ValueString())
		}
	})

	t.Run("slice_id takes precedence over id", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		_, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{
			SliceID: types.StringValue("slice-1"),
			ID:      types.StringValue("id-1"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if !containsCall(client.Calls, "GetSlice:slice-1") || containsCall(client.Calls, "GetSlice:id-1") {
			t.Fatalf("expected lookup by slice_id only, got %#v", client.Calls)
		}
	})

	t.Run("canonicalizes returned lease times", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{
			GetFn: func(_ context.Context, id string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: id, Name: "leased", State: "StableOK", LeaseStartTime: "2026-05-30T19:04:54Z", LeaseEndTime: "2026-05-31T19:04:54Z"}, nil
			},
		}
		got, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{SliceID: types.StringValue("slice-1")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if got.LeaseStartTime.ValueString() != "2026-05-30 19:04:54 +00:00" {
			t.Fatalf("lease_start_time = %q, want canonical FABRIC layout", got.LeaseStartTime.ValueString())
		}
		if got.LeaseEndTime.ValueString() != "2026-05-31 19:04:54 +00:00" {
			t.Fatalf("lease_end_time = %q, want canonical FABRIC layout", got.LeaseEndTime.ValueString())
		}
	})

	t.Run("invalid lease time errors", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{
			GetFn: func(_ context.Context, id string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: id, Name: "bad", State: "StableOK", LeaseStartTime: "not-a-time"}, nil
			},
		}
		_, resp := readSliceDataSource(t, ctx, client, SliceDataSourceModel{SliceID: types.StringValue("slice-1")})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error for an unparseable lease_start_time")
		}
	})
}

func readSliceDataSource(t *testing.T, ctx context.Context, client *fake.Client, model SliceDataSourceModel) (SliceDataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	d := &SliceDataSource{client: client}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: sliceDataSourceSchema(ctx)}}
	d.Read(ctx, datasource.ReadRequest{Config: configFromModel(t, ctx, sliceDataSourceSchema(ctx), model)}, resp)
	var got SliceDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading state: %v", diags)
		}
	}
	return got, resp
}
