package datasource

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

func metricsSchema(ctx context.Context) dschema.Schema {
	var sr datasource.SchemaResponse
	(&MetricsDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &sr)
	return sr.Schema
}

func metricsConfig(t *testing.T, ctx context.Context, model MetricsDataSourceModel) tfsdk.Config {
	t.Helper()
	s := metricsSchema(ctx)
	state := &tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding metrics model: %v", diags)
	}
	return tfsdk.Config{Schema: s, Raw: state.Raw}
}

func readMetrics(t *testing.T, ctx context.Context, client *fake.Client, model MetricsDataSourceModel) (MetricsDataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	d := &MetricsDataSource{client: client}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: metricsSchema(ctx)}}
	d.Read(ctx, datasource.ReadRequest{Config: metricsConfig(t, ctx, model)}, resp)
	var got MetricsDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading metrics state: %v", diags)
		}
	}
	return got, resp
}

func TestFabric_DataSource_Metrics_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("maps filters and results", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.MetricsQuery
		client := &fake.Client{MetricsFn: func(_ context.Context, query fabricclient.MetricsQuery) (string, error) {
			gotQuery = query
			return `[{"project":"p1"}]`, nil
		}}
		got, resp := readMetrics(t, ctx, client, MetricsDataSourceModel{
			ExcludedProjects: []types.String{types.StringValue("p2"), types.StringValue("p3")},
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if len(gotQuery.ExcludedProjects) != 2 || gotQuery.ExcludedProjects[0] != "p2" || gotQuery.ExcludedProjects[1] != "p3" {
			t.Fatalf("excluded_projects = %#v, want p2,p3", gotQuery.ExcludedProjects)
		}
		if got.Results.ValueString() != `[{"project":"p1"}]` {
			t.Fatalf("results = %q, want JSON", got.Results.ValueString())
		}
	})

	t.Run("client error becomes diagnostic", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{MetricsFn: func(context.Context, fabricclient.MetricsQuery) (string, error) {
			return "", errors.New("boom")
		}}
		_, resp := readMetrics(t, ctx, client, MetricsDataSourceModel{})
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected error diagnostic when client fails")
		}
	})
}
