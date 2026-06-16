package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// Component axis — only node-local components reach StableOK in testmode: GPU
// (RTX6000/Tesla T4 per RENCI-ad.graphml, not A30/A40) and NVMe (P4510). FullToken
// carries the matching tag.
//
// Same-site L2 network services now apply end-to-end in testmode (broker SharedNIC
// allocation and the NetworkServiceInventory control are fixed), covered by
// TestAccFabric_Slice_L2Bridge_Apply. FABNetv4 and other routed services stay
// plan-only: the testmode network AM advertises only L2STS/L2Bridge/L2PTP, so there
// is no routed-service inventory to satisfy them. Over-capacity is likewise not
// apply-testable: a declined node reservation hangs rather than settling to
// StableError. The plan-only tests below drive ModifyPlan through the real provider
// server for the types that cannot apply.

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
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				// Topology-specific: the ASM must carry the requested GPU component.
				testutil.CheckSliceGraph("fabric_slice.test", func(g *testutil.SliceGraph) error {
					ok, err := g.HasComponent("vm1", sliver.ComponentTypeGPU, "RTX6000")
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("ASM node vm1 missing GPU/RTX6000 component")
					}
					return nil
				}),
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
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				testutil.CheckSliceGraph("fabric_slice.test", func(g *testutil.SliceGraph) error {
					ok, err := g.HasComponent("vm1", sliver.ComponentTypeNVME, "P4510")
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("ASM node vm1 missing NVME/P4510 component")
					}
					return nil
				}),
			),
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
				// StandardSliceGraphChecks iterates every VM node, so it proves both
				// vm1 (RENC) and vm2 (UKY) allocated; CheckReservationMatchesSliverID
				// cross-checks each node's sliver_id, subsuming the per-node AttrSet.
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				testutil.CheckSliceGraph("fabric_slice.test", func(g *testutil.SliceGraph) error {
					vm1, err := g.Node("vm1")
					if err != nil {
						return err
					}
					vm2, err := g.Node("vm2")
					if err != nil {
						return err
					}
					if vm1.Site == vm2.Site {
						return fmt.Errorf("expected vm1 and vm2 at different sites, both at %q", vm1.Site)
					}
					return nil
				}),
			),
		}},
	})
}
