package datasource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fake "github.com/Testbed-IAC/fabric-go-fim/pkg/client/clienttest"
)

func advertisedFixture(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "fabric-go-fim", "testdata", "fixtures", "advertised_topology.graphml"))
	if err != nil {
		t.Fatalf("reading advertised fixture: %v", err)
	}
	return string(body)
}

func sitesSchema(ctx context.Context) dschema.Schema {
	var sr datasource.SchemaResponse
	(&SitesDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &sr)
	return sr.Schema
}

func facilityPortsSchema(ctx context.Context) dschema.Schema {
	var sr datasource.SchemaResponse
	(&FacilityPortsDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &sr)
	return sr.Schema
}

func sitesConfig(t *testing.T, ctx context.Context, model SitesDataSourceModel) tfsdk.Config {
	t.Helper()
	s := sitesSchema(ctx)
	state := &tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding sites model: %v", diags)
	}
	return tfsdk.Config{Schema: s, Raw: state.Raw}
}

func facilityPortsConfig(t *testing.T, ctx context.Context, model FacilityPortsDataSourceModel) tfsdk.Config {
	t.Helper()
	s := facilityPortsSchema(ctx)
	state := &tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding facility ports model: %v", diags)
	}
	return tfsdk.Config{Schema: s, Raw: state.Raw}
}

func readSites(t *testing.T, ctx context.Context, client *fake.Client, model SitesDataSourceModel) (SitesDataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	d := &SitesDataSource{client: client}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: sitesSchema(ctx)}}
	d.Read(ctx, datasource.ReadRequest{Config: sitesConfig(t, ctx, model)}, resp)
	var got SitesDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading sites state: %v", diags)
		}
	}
	return got, resp
}

func readFacilityPorts(t *testing.T, ctx context.Context, client *fake.Client, model FacilityPortsDataSourceModel) (FacilityPortsDataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	d := &FacilityPortsDataSource{client: client}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: facilityPortsSchema(ctx)}}
	d.Read(ctx, datasource.ReadRequest{Config: facilityPortsConfig(t, ctx, model)}, resp)
	var got FacilityPortsDataSourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading facility ports state: %v", diags)
		}
	}
	return got, resp
}

func TestFabric_DataSource_Sites_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := advertisedFixture(t)

	t.Run("maps advertised site fields", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.ResourcesQuery
		client := &fake.Client{ResourceFn: func(_ context.Context, query fabricclient.ResourcesQuery) (string, error) {
			gotQuery = query
			return fixture, nil
		}}
		got, resp := readSites(t, ctx, client, SitesDataSourceModel{
			Includes:     types.StringValue("RENC,UKY"),
			ForceRefresh: types.BoolValue(true),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotQuery.Level != advertisedResourcesLevel || gotQuery.Includes != "RENC,UKY" || !gotQuery.ForceRefresh {
			t.Fatalf("resources query = %+v, want level 2 with includes and force_refresh", gotQuery)
		}
		if len(got.Sites) != 1 {
			t.Fatalf("sites = %d, want 1", len(got.Sites))
		}
		site := got.Sites[0]
		if site.Name.ValueString() != "RENC" || !site.PTP.ValueBool() || !site.IPv4Management.ValueBool() {
			t.Fatalf("site = %+v, want RENC with flags", site)
		}
		if site.Cores.Capacity.ValueInt64() != 100 || site.Cores.Available.ValueInt64() != 90 {
			t.Fatalf("cores = %+v, want capacity 100 and available 90", site.Cores)
		}
		if len(site.Hosts) != 1 || site.Hosts[0].Name.ValueString() != "renc-w1" {
			t.Fatalf("hosts = %+v, want renc-w1", site.Hosts)
		}
		if len(site.Components) != 1 || site.Components[0].Name.ValueString() != "GPU/A30" {
			t.Fatalf("components = %+v, want GPU/A30", site.Components)
		}
	})

	t.Run("filters by name and excludes", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{ResourceFn: func(context.Context, fabricclient.ResourcesQuery) (string, error) {
			return fixture, nil
		}}
		got, resp := readSites(t, ctx, client, SitesDataSourceModel{Name: types.StringValue("RENC"), Excludes: types.StringValue("RENC")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if len(got.Sites) != 0 {
			t.Fatalf("sites = %d, want 0 after exclude", len(got.Sites))
		}
	})
}

func TestFabric_DataSource_FacilityPorts_Read(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := advertisedFixture(t)

	t.Run("maps advertised facility port fields", func(t *testing.T) {
		t.Parallel()
		var gotQuery fabricclient.ResourcesQuery
		client := &fake.Client{ResourceFn: func(_ context.Context, query fabricclient.ResourcesQuery) (string, error) {
			gotQuery = query
			return fixture, nil
		}}
		got, resp := readFacilityPorts(t, ctx, client, FacilityPortsDataSourceModel{
			Includes: types.StringValue("RENC"),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if gotQuery.Level != advertisedResourcesLevel || gotQuery.Includes != "RENC" {
			t.Fatalf("resources query = %+v, want level 2 with includes", gotQuery)
		}
		if len(got.FacilityPorts) != 1 {
			t.Fatalf("facility_ports = %d, want 1", len(got.FacilityPorts))
		}
		port := got.FacilityPorts[0]
		if port.Name.ValueString() != "RENC-ESnet" || port.Site.ValueString() != "RENC" || port.Switch.ValueString() != "renc-data-sw" {
			t.Fatalf("facility port = %+v, want decoded identity fields", port)
		}
		if port.VLANRange.ValueString() != "100-200" || port.Bandwidth.ValueInt64() != 100 {
			t.Fatalf("facility port = %+v, want vlan range and bandwidth", port)
		}
	})

	t.Run("filters by name and excludes", func(t *testing.T) {
		t.Parallel()
		client := &fake.Client{ResourceFn: func(context.Context, fabricclient.ResourcesQuery) (string, error) {
			return fixture, nil
		}}
		got, resp := readFacilityPorts(t, ctx, client, FacilityPortsDataSourceModel{Name: types.StringValue("RENC-ESnet"), Excludes: types.StringValue("RENC")})
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics.Errors())
		}
		if len(got.FacilityPorts) != 0 {
			t.Fatalf("facility_ports = %d, want 0 after exclude", len(got.FacilityPorts))
		}
	})
}
