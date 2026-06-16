package provider_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestAccFabric_Slice_NodeCapacityAndImage applies a VM with explicit
// cores/ram/disk and a non-default image, then verifies the requested capacities
// and image survive into the orchestrator ASM. (Exercises the WithCapacity and
// WithImage config options, which no prior apply test used.)
func TestAccFabric_Slice_NodeCapacityAndImage(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.BareVMConfig(name,
				testutil.WithCapacity(4, 16, 20), testutil.WithImage("default_ubuntu_22")),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				testutil.CheckSliceGraph("fabric_slice.test", func(g *testutil.SliceGraph) error {
					vm1, err := g.Node("vm1")
					if err != nil {
						return err
					}
					if vm1.ImageRef != "default_ubuntu_22" {
						return fmt.Errorf("ImageRef = %q, want default_ubuntu_22", vm1.ImageRef)
					}
					if vm1.Capacities == nil {
						return fmt.Errorf("node vm1 has no Capacities")
					}
					if vm1.Capacities.Core != 4 || vm1.Capacities.RAM != 16 || vm1.Capacities.Disk != 20 {
						return fmt.Errorf("Capacities = %+v, want core=4 ram=16 disk=20", *vm1.Capacities)
					}
					return nil
				}),
			),
		}},
	})
}

// TestAccFabric_Slice_NodeUserData applies a VM that sets boot_script,
// post_boot_execute, and post_update, proving the node user-data fields build a
// valid topology and reach StableOK. (No prior test exercised these fields.)
func TestAccFabric_Slice_NodeUserData(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.NodeUserDataConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
			),
		}},
	})
}

// TestAccFabric_Slice_LeaseStartTime verifies the provider accepts and plans an
// explicit lease_start_time. It is plan-only by necessity: the orchestrator
// rejects a start under 60 minutes out ("Requested Start Time should be at least
// 60 minutes from the current time"), and a start far enough out to satisfy that
// would also defer redemption past the test timeout. The value is supplied in
// canonical FABRIC time format ("2006-01-02 15:04:05 -07:00"); a non-canonical
// form (e.g. RFC3339) is rewritten by ModifyPlan and Terraform then rejects the
// plan as inconsistent with config, so the canonical form is what an author must
// provide.
func TestAccFabric_Slice_LeaseStartTime(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	start := time.Now().UTC().Add(2 * time.Hour).Format("2006-01-02 15:04:05 -07:00")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:             testutil.ProviderConfig(token) + testutil.LeaseStartConfig(name, start),
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
		}},
	})
}
