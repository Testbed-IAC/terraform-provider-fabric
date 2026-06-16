package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// Plan-only topology tests. Each drives a topology block through the real
// provider server's ModifyPlan (catalog validation, topology build, permission
// tags, validators) but does not apply — the testmode network-service
// reservation hangs on apply, and these blocks need only prove they plan cleanly.
// Pattern matches the existing TestAccFabric_Slice_L2Bridge_Plan: PlanOnly with
// ExpectNonEmptyPlan.

func planOnlyStep(token, body string) resource.TestStep {
	return resource.TestStep{
		Config:             testutil.ProviderConfig(token) + body,
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	}
}

func runPlanOnlyCase(t *testing.T, bodyFmt string) {
	t.Helper()
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps:                    []resource.TestStep{planOnlyStep(token, fmt.Sprintf(bodyFmt, name))},
	})
}

func TestAccFabric_Slice_Labels_Plan(t *testing.T) {
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    labels = {
      instance_parent = "renc-w1"
    }
  }
}
`)
}

func TestAccFabric_Slice_Route_Plan(t *testing.T) {
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    route {
      subnet   = "10.20.0.0/24"
      next_hop = "10.20.0.1"
    }
  }
}
`)
}

func TestAccFabric_Slice_Storage_Plan(t *testing.T) {
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    storage {
      name       = "vol1"
      auto_mount = true
    }
  }
}
`)
}

func TestAccFabric_Slice_Switch_Plan(t *testing.T) {
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
  }

  switch {
    name   = "sw1"
    site   = "RENC"
    nports = 4
  }
}
`)
}

func TestAccFabric_Slice_FacilityPort_Plan(t *testing.T) {
	// Mirrors internal/slice/testdata/facility_l2sts_smartnic_port.graphml: a VM
	// SmartNIC port (UKY) and a facility port (RENC) joined by an L2STS service.
	// L2STS is a cross-site service, so the node and facility sit at two sites.
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "UKY"
    image_ref = "default_rocky_9"
    component {
      name  = "snic1"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  facility_port {
    name = "RENCI-DTN"
    site = "RENC"
    vlan = "100"
  }

  network {
    name = "s-fac"
    type = "L2STS"
    interface {
      node      = "vm1"
      component = "snic1"
    }
    interface {
      facility = "RENCI-DTN"
    }
  }
}
`)
}

func TestAccFabric_Slice_SubInterface_Plan(t *testing.T) {
	// A SmartNIC DedicatedPort carrying a VLAN sub-interface, joined to a second
	// node's SmartNIC by an L2Bridge.
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    component {
      name  = "snic1"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name      = "vm2"
    site      = "RENC"
    image_ref = "default_rocky_9"
    component {
      name  = "snic2"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "net1"
    type = "L2Bridge"
    interface {
      node      = "vm1"
      component = "snic1"
      sub_interface {
        name = "vlan100"
        vlan = "100"
      }
    }
    interface {
      node      = "vm2"
      component = "snic2"
    }
  }
}
`)
}

func TestAccFabric_Slice_NetworkGateway_Plan(t *testing.T) {
	// A routed FABNetv4 service with an explicit gateway.
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "net1"
    type = "FABNetv4"
    interface {
      node      = "vm1"
      component = "nic1"
    }
    gateway = {
      ipv4        = "10.130.1.1"
      ipv4_subnet = "10.130.1.0/24"
    }
  }
}
`)
}

func TestAccFabric_Slice_PortMirror_Plan(t *testing.T) {
	// A PortMirror service mirroring a SmartNIC port. mirror_from names an
	// existing component port; the service's own interface is the destination.
	runPlanOnlyCase(t, `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["`+testutil.DummySSHKey+`"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    component {
      name  = "snic1"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
    component {
      name  = "snic2"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name             = "mirror1"
    type             = "PortMirror"
    site             = "RENC"
    mirror_from      = "snic1-p1"
    mirror_direction = "Both"
    interface {
      node      = "vm1"
      component = "snic2"
    }
  }
}
`)
}

// --- Apply tests for scenarios that were once fork-blocked (now fixed) -------

// TestAccFabric_Slice_TopologyModify exercises the real ModifySlice path: it
// applies a single-VM slice, then scales it up to two VMs, which the provider's
// Update submits as a topology modify (not a lease renew). This was previously
// blocked by an orchestrator bug (orchestrator_slice_wrapper.py read
// reservation_info from the freshly built modify graph, raising
// AttributeError: 'NoneType' has no 'reservation_id'); the fork now resolves the
// reservation id from the existing slice by node name. StandardSliceGraphChecks
// validates every VM node, so it proves the modify-added vm2 actually allocated.
func TestAccFabric_Slice_TopologyModify(t *testing.T) {
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
					resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
			{
				Config: testutil.ProviderConfig(token) + testutil.TwoSiteNodesConfig(name, testutil.TestSite, testutil.TestSite),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
					resource.TestCheckResourceAttrSet("fabric_slice.test", "nodes.vm2.sliver_id"),
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
		},
	})
}

// TestAccFabric_Slice_L2Bridge_Apply applies (not just plans) a two-node L2Bridge
// and asserts the slice reaches StableOK with both VMs allocated. This was
// previously blocked in testmode by broker bugs in the network path — a SharedNIC
// allocation crash (network_node_inventory.py dereferenced an absent numa
// delegation) and a missing NetworkServiceInventory control in the broker config
// (network-service types fell back to NetworkNodeInventory). Both are fixed in
// the fork; the network AM's MockAMHandler then drives the service to Active.
func TestAccFabric_Slice_L2Bridge_Apply(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + testutil.TwoNodeNetworkConfig(name, "L2Bridge", "", ""),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_slice.test", "state", "StableOK"),
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				// The L2Bridge network service must be present in the ASM.
				testutil.CheckSliceGraph("fabric_slice.test", func(g *testutil.SliceGraph) error {
					for _, n := range g.NetworkServiceNames() {
						if n == "net1" {
							return nil
						}
					}
					return fmt.Errorf("ASM missing the net1 network service; have %v", g.NetworkServiceNames())
				}),
			),
		}},
	})
}
