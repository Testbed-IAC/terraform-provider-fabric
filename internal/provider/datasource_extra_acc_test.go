package provider_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestAccFabric_DataSource_Slice_BySliceID looks a slice up through the
// fabric_slice data source by slice_id (the highest-precedence lookup key, never
// previously covered — the existing test uses name).
func TestAccFabric_DataSource_Slice_BySliceID(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.SliceWithSliceLookupsConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.fabric_slice.by_slice_id", "slice_id", "fabric_slice.test", "slice_id"),
				resource.TestCheckResourceAttrPair("data.fabric_slice.by_slice_id", "name", "fabric_slice.test", "name"),
				resource.TestCheckResourceAttr("data.fabric_slice.by_slice_id", "state", "StableOK"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
			),
		}},
	})
}

// TestAccFabric_DataSource_Slice_ByID looks a slice up through the fabric_slice
// data source by id (the second-precedence lookup key).
func TestAccFabric_DataSource_Slice_ByID(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.SliceWithSliceLookupsConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.fabric_slice.by_id", "slice_id", "fabric_slice.test", "slice_id"),
				resource.TestCheckResourceAttrPair("data.fabric_slice.by_id", "name", "fabric_slice.test", "name"),
				resource.TestCheckResourceAttr("data.fabric_slice.by_id", "state", "StableOK"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
			),
		}},
	})
}

// TestAccFabric_DataSource_Slice_ComputedAttrs asserts the fabric_slice data
// source returns the computed graph_id and lease times (never previously
// asserted).
func TestAccFabric_DataSource_Slice_ComputedAttrs(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.SliceWithSliceLookupsConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("data.fabric_slice.by_slice_id", "graph_id"),
				resource.TestCheckResourceAttrSet("data.fabric_slice.by_slice_id", "lease_start_time"),
				resource.TestCheckResourceAttrSet("data.fabric_slice.by_slice_id", "lease_end_time"),
				resource.TestCheckResourceAttrPair("data.fabric_slice.by_slice_id", "graph_id", "fabric_slice.test", "graph_id"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
			),
		}},
	})
}

// TestAccFabric_DataSource_Slivers reads per-sliver state for a provisioned slice
// through the fabric_slivers data source. This data source reads orchestrator
// slivers directly (it does not hit the advertised-resources decoder gap), so it
// is fully testable in testmode. A bare VM yields exactly one NodeSliver.
func TestAccFabric_DataSource_Slivers(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.SliceWithSliversDataSourceConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.fabric_slivers.all", "slivers.#", "1"),
				resource.TestCheckResourceAttr("data.fabric_slivers.all", "slivers.0.sliver_type", "NodeSliver"),
				resource.TestCheckResourceAttr("data.fabric_slivers.all", "slivers.0.state", "Active"),
				resource.TestCheckResourceAttrSet("data.fabric_slivers.all", "slivers.0.sliver_id"),
				resource.TestCheckResourceAttrPair("data.fabric_slivers.all", "slivers.0.sliver_id", "fabric_slice.test", "nodes.vm1.sliver_id"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
			),
		}},
	})
}

// TestAccFabric_DataSource_Resources_Options reads available resources at level 2
// with force_refresh, exercising the resources data source options (the existing
// test only covers level 1 defaults).
func TestAccFabric_DataSource_Resources_Options(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.ResourcesOptionsDataSourceConfig(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("data.fabric_resources.lvl2", "model"),
				resource.TestCheckResourceAttr("data.fabric_resources.lvl2", "level", "2"),
				resource.TestCheckResourceAttr("data.fabric_resources.lvl2", "force_refresh", "true"),
			),
		}},
	})
}

// TestAccFabric_DataSource_Metrics reads the orchestrator metrics overview. The
// testmode orchestrator serves GET /metrics/overview from its Postgres slice
// counts, so results is a non-empty JSON array containing a "slices" object.
func TestAccFabric_DataSource_Metrics(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.MetricsDataSourceConfig(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.fabric_metrics.overview", "id", "metrics"),
				resource.TestCheckResourceAttrSet("data.fabric_metrics.overview", "results"),
				resource.TestMatchResourceAttr("data.fabric_metrics.overview", "results", regexp.MustCompile(`slices`)),
			),
		}},
	})
}

// TestAccFabric_DataSource_Sites reads the typed sites data source. Against the
// testmode broker CBM, catalog.DecodeAdvertised returns zero sites (a documented
// model-format gap: the CBM advertises Server workers with a site property but no
// aggregating Type="Site" node). The data source must still decode gracefully to
// an empty list with no error.
func TestAccFabric_DataSource_Sites(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.SitesDataSourceConfig(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.fabric_sites.all", "id", "sites"),
				resource.TestCheckResourceAttr("data.fabric_sites.all", "sites.#", "0"),
			),
		}},
	})
}

// TestAccFabric_DataSource_FacilityPorts reads the typed facility-ports data
// source. Unlike fabric_sites (whose aggregating Type="Site" node is absent from
// the testmode CBM), the network AM's Network-ad.graphml advertises FacilityPort
// nodes, so catalog.DecodeAdvertised returns a non-empty list here. The test
// asserts the decoder produces well-formed entries rather than a fixed count.
func TestAccFabric_DataSource_FacilityPorts(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.FacilityPortsDataSourceConfig(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.fabric_facility_ports.all", "id", "facility_ports"),
				resource.TestCheckResourceAttrWith("data.fabric_facility_ports.all", "facility_ports.#", func(value string) error {
					n, err := strconv.Atoi(value)
					if err != nil {
						return fmt.Errorf("facility_ports.# = %q: %w", value, err)
					}
					if n < 1 {
						return fmt.Errorf("facility_ports.# = %d, want >= 1 (testmode Network-ad advertises facility ports)", n)
					}
					return nil
				}),
				resource.TestCheckResourceAttrSet("data.fabric_facility_ports.all", "facility_ports.0.name"),
				resource.TestCheckResourceAttrSet("data.fabric_facility_ports.all", "facility_ports.0.site"),
			),
		}},
	})
}
