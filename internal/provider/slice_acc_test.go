package provider_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// Slice lifecycle acceptance tests run against the local testmode stack. The
// orchestrator is backed by a single Postgres/Neo4j, so these are NOT run in parallel
// to avoid contention on shared state. Each test mints its own token and names its
// slice uniquely.

func TestAccFabric_Slice_BasicLifecycle(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(token) + testutil.BareVMConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fabric_slice.test", "slice_id"),
					// state is the slice-envelope field, not an ASM property, so the graph
					// checks cannot prove it — assert it directly.
					resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
					// Graph verification subsumes the per-node management_ip/sliver_id
					// attribute checks: it proves the ASM VM node exists with a MgmtIp and
					// an Active reservation whose id equals nodes.vm1.sliver_id.
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
			{
				Config:             testutil.ProviderConfig(token) + testutil.BareVMConfig(name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccFabric_Slice_Update exercises the provider's in-place update via the lease
// RENEW path (changing lifetime_hours with identical topology). It deliberately keeps
// the topology identical so this stays a pure renew test; the topology-modify path
// (ModifySlice) is covered separately by TestAccFabric_Slice_TopologyModify.
func TestAccFabric_Slice_Update(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{Config: testutil.ProviderConfig(token) + testutil.BareVMConfig(name, testutil.WithLifetime(24))},
			{
				Config: testutil.ProviderConfig(token) + testutil.BareVMConfig(name, testutil.WithLifetime(48)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabric_slice.test", "lifetime_hours", "48"),
					// The renew path keeps the topology identical; the ASM must still
					// describe an allocated VM after the in-place update.
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
			{
				Config:             testutil.ProviderConfig(token) + testutil.BareVMConfig(name, testutil.WithLifetime(48)),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccFabric_Slice_Import verifies that importing a slice by ID recovers its
// identity. The provider's ImportState sets slice_id and Read repopulates the slice's
// server-derived attributes; it cannot reconstruct the config-owned `node` blocks (and
// the computed `nodes` map derives from them), so a full ImportStateVerify round-trip is
// not possible by design. We assert the identity attributes import recovers instead.
func TestAccFabric_Slice_Import(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(token) + testutil.BareVMConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
			{
				ResourceName: "fabric_slice.test",
				ImportState:  true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported instance, got %d", len(states))
					}
					attrs := states[0].Attributes
					for _, key := range []string{"slice_id", "id", "name", "state"} {
						if attrs[key] == "" {
							return fmt.Errorf("imported state missing %q", key)
						}
					}
					if attrs["name"] != name {
						return fmt.Errorf("imported name = %q, want %q", attrs["name"], name)
					}
					if attrs["state"] != "StableOK" {
						return fmt.Errorf("imported state = %q, want StableOK", attrs["state"])
					}
					return nil
				},
			},
		},
	})
}

// TestAccFabric_Slice_Disappears deletes the slice out-of-band after apply and asserts
// Terraform detects the drift (non-empty plan to recreate). The canonical single-step
// disappears pattern: the delete runs in the step's Check, and ExpectNonEmptyPlan marks
// the post-apply refresh plan as expected-non-empty.
func TestAccFabric_Slice_Disappears(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(token) + testutil.BareVMConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fabric_slice.test", "slice_id"),
					// Verify the ASM while the slice still exists, then delete it
					// out-of-band. ComposeAggregate runs checks in order, so the graph
					// fetch happens before the delete.
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
					testAccDeleteSliceOutOfBand("fabric_slice.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func checkAccSliceDestroy(state *terraform.State) error {
	client, err := testutil.Client()
	if err != nil {
		return err
	}
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "fabric_slice" {
			continue
		}
		sliceID := rs.Primary.Attributes["slice_id"]
		if sliceID == "" {
			continue
		}
		slice, err := client.GetSlice(context.Background(), sliceID)
		if errors.Is(err, fabricclient.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking slice %s destroy: %w", sliceID, err)
		}
		// A deleted FABRIC slice is not 404'd immediately; it transitions to a terminal
		// Dead/Closing state and is garbage-collected later. Treat those as destroyed.
		if isDestroyedState(slice.State) {
			continue
		}
		return fmt.Errorf("slice %s still exists in state %q", sliceID, slice.State)
	}
	return nil
}

// isDestroyedState reports whether a slice state means the slice is gone or on its way
// out (no live resources remain).
func isDestroyedState(state string) bool {
	switch state {
	case "Dead", "Closing":
		return true
	default:
		return false
	}
}

func testAccDeleteSliceOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		client, err := testutil.Client()
		if err != nil {
			return err
		}
		rs, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		sliceID := rs.Primary.Attributes["slice_id"]
		if sliceID == "" {
			return fmt.Errorf("resource %s has no slice_id", resourceName)
		}
		ctx := context.Background()
		if err := client.DeleteSlice(ctx, sliceID); err != nil && !errors.Is(err, fabricclient.ErrNotFound) {
			return fmt.Errorf("deleting slice %s out of band: %w", sliceID, err)
		}
		// Deletion is async (Active -> Closing -> Dead). Wait until the slice is
		// terminally gone so the subsequent refresh deterministically observes the
		// drift (the provider's Read removes a Dead/missing slice from state). Without
		// this, the refresh can race a still-Closing slice and see no drift.
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			slice, err := client.GetSlice(ctx, sliceID)
			if errors.Is(err, fabricclient.ErrNotFound) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("polling out-of-band deleted slice %s: %w", sliceID, err)
			}
			if isDestroyedState(slice.State) {
				return nil
			}
			time.Sleep(3 * time.Second)
		}
		return fmt.Errorf("slice %s did not reach a terminal deleted state within timeout", sliceID)
	}
}
