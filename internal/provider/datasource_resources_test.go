package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fake "github.com/Testbed-IAC/fabric-go-fim/pkg/client/clienttest"
)

func resourcesSchema(ctx context.Context) dschema.Schema {
	var sr datasource.SchemaResponse
	(&ResourcesDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &sr)
	return sr.Schema
}

func resourcesConfig(t *testing.T, ctx context.Context, model ResourcesDataSourceModel) tfsdk.Config {
	t.Helper()
	s := resourcesSchema(ctx)
	state := &tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding resources model: %v", diags)
	}
	return tfsdk.Config{Schema: s, Raw: state.Raw}
}

func readResources(t *testing.T, ctx context.Context, client *fake.Client, model ResourcesDataSourceModel) (ResourcesDataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	d := &ResourcesDataSource{client: client}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: resourcesSchema(ctx)}}
	d.Read(ctx, datasource.ReadRequest{Config: resourcesConfig(t, ctx, model)}, resp)
	var got ResourcesDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading state: %v", diags)
		}
	}
	return got, resp
}

func TestFabric_DataSource_Resources_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("defaults level to 1", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.ResourcesQuery
		client := &fake.Client{ResourceFn: func(_ context.Context, query fabricclient.ResourcesQuery) (string, error) {
			gotQuery = query
			return "<graphml/>", nil
		}}
		got, resp := readResources(t, ctx, client, ResourcesDataSourceModel{})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotQuery.Level != 1 {
			t.Errorf("level passed to client = %d, want 1 (default)", gotQuery.Level)
		}
		if gotQuery.ForceRefresh {
			t.Errorf("force_refresh passed to client = true, want false")
		}
		if got.Model.ValueString() != "<graphml/>" {
			t.Errorf("model = %q, want <graphml/>", got.Model.ValueString())
		}
		if got.Level.ValueInt64() != 1 {
			t.Errorf("level = %d, want 1", got.Level.ValueInt64())
		}
	})

	t.Run("passes force_refresh through", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.ResourcesQuery
		client := &fake.Client{ResourceFn: func(_ context.Context, query fabricclient.ResourcesQuery) (string, error) {
			gotQuery = query
			return "<graphml/>", nil
		}}
		_, resp := readResources(t, ctx, client, ResourcesDataSourceModel{
			Level:        types.Int64Value(2),
			ForceRefresh: types.BoolValue(true),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotQuery.Level != 2 {
			t.Errorf("level passed to client = %d, want 2", gotQuery.Level)
		}
		if !gotQuery.ForceRefresh {
			t.Errorf("force_refresh passed to client = false, want true")
		}
	})

	t.Run("passes filters through", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.ResourcesQuery
		client := &fake.Client{ResourceFn: func(_ context.Context, query fabricclient.ResourcesQuery) (string, error) {
			gotQuery = query
			return "<graphml/>", nil
		}}
		got, resp := readResources(t, ctx, client, ResourcesDataSourceModel{
			Level:     types.Int64Value(2),
			StartDate: types.StringValue("2026-05-30T19:04:54Z"),
			EndDate:   types.StringValue("2026-05-31 19:04:54 +00:00"),
			Includes:  types.StringValue("RENC,UKY"),
			Excludes:  types.StringValue("STAR"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotQuery.StartDate != "2026-05-30 19:04:54 +00:00" || gotQuery.EndDate != "2026-05-31 19:04:54 +00:00" {
			t.Fatalf("date filters = %q/%q, want canonical FABRIC times", gotQuery.StartDate, gotQuery.EndDate)
		}
		if gotQuery.Includes != "RENC,UKY" || gotQuery.Excludes != "STAR" {
			t.Fatalf("site filters = includes:%q excludes:%q", gotQuery.Includes, gotQuery.Excludes)
		}
		if got.StartDate.ValueString() != gotQuery.StartDate {
			t.Fatalf("start_date state = %q, want %q", got.StartDate.ValueString(), gotQuery.StartDate)
		}
	})

	t.Run("uses portal resources when graph_format is set", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.ResourcesQuery
		client := &fake.Client{
			ResourceFn: func(context.Context, fabricclient.ResourcesQuery) (string, error) {
				t.Fatal("GetResources must not be called when graph_format is set")
				return "", nil
			},
			PortalResourceFn: func(_ context.Context, query fabricclient.ResourcesQuery) (string, error) {
				gotQuery = query
				return "<portal/>", nil
			},
		}
		got, resp := readResources(t, ctx, client, ResourcesDataSourceModel{
			GraphFormat: types.StringValue("GRAPHML"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotQuery.GraphFormat != "GRAPHML" {
			t.Fatalf("graph_format = %q, want GRAPHML", gotQuery.GraphFormat)
		}
		if got.Model.ValueString() != "<portal/>" {
			t.Fatalf("model = %q, want portal response", got.Model.ValueString())
		}
	})

	t.Run("client error becomes diagnostic", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{ResourceFn: func(context.Context, fabricclient.ResourcesQuery) (string, error) {
			return "", errors.New("boom")
		}}
		_, resp := readResources(t, ctx, client, ResourcesDataSourceModel{})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error diagnostic when client fails")
		}
	})
}
