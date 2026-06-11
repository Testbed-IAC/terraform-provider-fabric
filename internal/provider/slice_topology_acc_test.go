package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// Component axis — only node-local components reach StableOK in testmode: GPU
// (RTX6000/Tesla T4 per RENCI-ad.graphml, not A30/A40) and NVMe (P4510). FullToken
// carries the matching tag.
//
// Anything that pulls in a network-service reservation is NOT apply-tested: NICs
// (SharedNIC/SmartNIC), explicit network blocks, and network-attached Storage (NAS). In
// this testmode build the network-service reservation fails at the broker and the
// controller relinquishes-and-retries it indefinitely, so the slice never reaches a
// terminal state (it hangs rather than erroring). Those types are covered by (a) the
// provider's unit golden-topology tests in internal/slice and (b) the plan-only tests
// below, which drive ModifyPlan through the real provider server. Over-capacity is
// likewise not apply-testable: a declined node reservation hangs the same way rather
// than settling to StableError. See ACCEPTANCE_TEST_PLAN.md §7.

func TestAccFabric_Slice_GPU(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.VMWithComponentConfig(name, "GPU", "RTX6000"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
				resource.TestCheckResourceAttrSet("fabric_slice.test", "nodes.vm1.sliver_id"),
			),
		}},
	})
}

func TestAccFabric_Slice_NVME(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.VMWithComponentConfig(name, "NVME", "P4510"),
			Check:  resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
		}},
	})
}

// Network axis — plan-only. These validate that the provider plans a network slice
// cleanly through the real provider server (catalog validation, topology build,
// permission tags, validators in ModifyPlan) for the types A3 found to have an
// advertised CBM owner. They do not apply: see the package note above on the testmode
// network-service hang.

func TestAccFabric_Slice_L2Bridge_Plan(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:             testutil.ProviderConfig(token) + testutil.TwoNodeNetworkConfig(name, "L2Bridge", "", ""),
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccFabric_Slice_FABNetv4_Plan(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:             testutil.ProviderConfig(token) + testutil.TwoNodeNetworkConfig(name, "FABNetv4", "", ""),
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
		}},
	})
}

// Multi-site axis — genuine apply test, gated on the §4f UKY site AM
// (FABRIC_TESTMODE_MULTISITE=1). Two node-only VMs at RENC and UKY (no network service,
// to avoid the testmode network-reservation hang) must both reach StableOK, proving
// real cross-site placement and the Slice.Multisite permission path end-to-end.
func TestAccFabric_Slice_Multisite(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	testutil.SkipIfNoMultisite(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.TwoSiteNodesConfig(name, testutil.TestSite, testutil.TestSiteUKY),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
				resource.TestCheckResourceAttrSet("fabric_slice.test", "nodes.vm1.sliver_id"),
				resource.TestCheckResourceAttrSet("fabric_slice.test", "nodes.vm2.sliver_id"),
			),
		}},
	})
}
