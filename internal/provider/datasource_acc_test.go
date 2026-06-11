package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestAccFabric_DataSource_Resources reads the testmode available-resources model.
// It only needs a valid token (no slice), and depends on the `claim` service having
// populated the broker CBM (gated in TestMain).
func TestAccFabric_DataSource_Resources(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(testutil.FullToken()) + `
data "fabric_resources" "all" {
  level = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fabric_resources.all", "model"),
					resource.TestCheckResourceAttr("data.fabric_resources.all", "level", "1"),
				),
			},
		},
	})
}

// TestAccFabric_DataSource_Slice provisions a slice and looks it up by name through the
// data source, verifying it resolves the same slice id and reaches StableOK.
func TestAccFabric_DataSource_Slice(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(token) + sliceWithDataSourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.fabric_slice.by_name", "slice_id",
						"fabric_slice.test", "slice_id",
					),
					resource.TestCheckResourceAttr("data.fabric_slice.by_name", "state", "StableOK"),
				),
			},
		},
	})
}

// NOTE: there is intentionally no fabric_sites / fabric_facility_ports acceptance test.
// Those data sources decode advertised resources via catalog.DecodeAdvertised, which
// keys sites off an aggregating GraphML node of Type "Site". The testmode broker CBM
// (built from RENCI-ad.graphml) does not contain that aggregating node — it advertises
// Server workers with a site property only — so DecodeAdvertised returns zero sites
// against testmode even though slices place on RENC correctly. This is a model-format
// gap in fabric-go-fim's advertised decoder vs. the live broker query model, not a
// provider-logic issue; those data sources are covered by unit tests in
// internal/datasource against the fixture model. See ACCEPTANCE_TEST_PLAN.md §2a / §7.

func sliceWithDataSourceConfig(name string) string {
	return fmt.Sprintf(`%s
data "fabric_slice" "by_name" {
  name = fabric_slice.test.name
}
`, testutil.BareVMConfig(name))
}
