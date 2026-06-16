package provider_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// poaOp describes one POA resource in the all-operations config: its resource
// label, the operation, and any operation-specific argument HCL.
type poaOp struct {
	label     string
	operation string
	extraHCL  string
}

// allPOAOps covers every operation accepted by the POA schema. MockAMHandler.poa
// (fabric_cf/actor/handlers/mock_am_handler.py) ignores the request payload and
// drives every operation to RESULT_CODE_OK, so each reaches state Success
// regardless of inputs; the operation-specific args here are the ones each
// operation's schema documents.
var allPOAOps = []poaOp{
	{"cpuinfo", "cpuinfo", "  node_set = [\"vm1\"]\n"},
	{"numainfo", "numainfo", ""},
	{"reboot", "reboot", ""},
	{"rescan", "rescan", ""},
	{"cpupin", "cpupin", "  vcpu_cpu_map = [{ vcpu = \"0\", cpu = \"0\" }]\n"},
	{"numatune", "numatune", "  vcpu_cpu_map = [{ vcpu = \"0\", cpu = \"1\" }]\n"},
	{"addkey", "addkey", fmt.Sprintf("  keys = [{ key = %q, comment = \"acc\" }]\n", testutil.DummySSHKey)},
	{"removekey", "removekey", fmt.Sprintf("  keys = [{ key = %q, comment = \"acc\" }]\n", testutil.DummySSHKey)},
}

func allPOAOpsConfig(name string) string {
	var b strings.Builder
	b.WriteString(testutil.BareVMConfig(name))
	// The orchestrator allows only one in-flight POA per sliver ("Reservation has
	// a pending operation"), so chain the POA resources with depends_on to force
	// sequential creation. The provider's Create already blocks until each POA
	// reaches terminal Success, so the reservation is free before the next starts.
	prev := ""
	for _, op := range allPOAOps {
		dependsOn := ""
		if prev != "" {
			dependsOn = fmt.Sprintf("  depends_on = [fabric_poa.%s]\n", prev)
		}
		fmt.Fprintf(&b, `
resource "fabric_poa" %q {
  sliver_id = fabric_slice.test.nodes["vm1"].sliver_id
  operation = %q
%s%s}
`, op.label, op.operation, op.extraHCL, dependsOn)
		prev = op.label
	}
	return b.String()
}

// TestAccFabric_POA_AllOperations runs every POA operation against one slice and
// asserts each reaches terminal Success with a poa_id (only reboot was previously
// covered).
func TestAccFabric_POA_AllOperations(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	checks := []resource.TestCheckFunc{
		testutil.StandardSliceGraphChecks("fabric_slice.test"),
		testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
	}
	for _, op := range allPOAOps {
		res := "fabric_poa." + op.label
		checks = append(checks,
			resource.TestCheckResourceAttr(res, "state", "Success"),
			resource.TestCheckResourceAttrSet(res, "poa_id"),
		)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + allPOAOpsConfig(name),
			Check:  resource.ComposeAggregateTestCheckFunc(checks...),
		}},
	})
}

// poaWithTimeoutConfig is a slice + a single reboot POA whose create timeout is
// set. Changing only the timeout between applies is an in-place change, which is
// the one way to make Terraform call the POA resource's Update (every request
// argument is RequiresReplace).
func poaWithTimeoutConfig(name, createTimeout string) string {
	return fmt.Sprintf(`%s
resource "fabric_poa" "reboot" {
  sliver_id = fabric_slice.test.nodes["vm1"].sliver_id
  operation = "reboot"
  timeouts {
    create = %q
  }
}
`, testutil.BareVMConfig(name), createTimeout)
}

// TestAccFabric_POA_UpdateUnsupported asserts the POA resource rejects in-place
// updates. The Update method (poa_resource.go) always errors; it is reached by
// changing the non-RequiresReplace timeouts block.
func TestAccFabric_POA_UpdateUnsupported(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(token) + poaWithTimeoutConfig(name, "10m"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabric_poa.reboot", "state", "Success"),
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
			{
				Config:      testutil.ProviderConfig(token) + poaWithTimeoutConfig(name, "11m"),
				ExpectError: regexp.MustCompile(`Update FABRIC POA unsupported`),
			},
		},
	})
}

// poaWithTriggersConfig is a slice + a reboot POA carrying a triggers map. The
// triggers map is RequiresReplace, so changing it replaces the POA and re-runs
// the operation, yielding a fresh poa_id.
func poaWithTriggersConfig(name, triggerValue string) string {
	return fmt.Sprintf(`%s
resource "fabric_poa" "reboot" {
  sliver_id = fabric_slice.test.nodes["vm1"].sliver_id
  operation = "reboot"
  triggers = {
    run = %q
  }
}
`, testutil.BareVMConfig(name), triggerValue)
}

// TestAccFabric_POA_TriggersForceReRun changes the triggers map between applies
// and asserts the POA is replaced (a new poa_id) and re-runs to Success.
func TestAccFabric_POA_TriggersForceReRun(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	var firstPOAID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig(token) + poaWithTriggersConfig(name, "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabric_poa.reboot", "state", "Success"),
					resource.TestCheckResourceAttrWith("fabric_poa.reboot", "poa_id", func(value string) error {
						if value == "" {
							return fmt.Errorf("poa_id is empty")
						}
						firstPOAID = value
						return nil
					}),
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
			{
				Config: testutil.ProviderConfig(token) + poaWithTriggersConfig(name, "v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabric_poa.reboot", "state", "Success"),
					resource.TestCheckResourceAttrWith("fabric_poa.reboot", "poa_id", func(value string) error {
						if value == firstPOAID {
							return fmt.Errorf("poa_id did not change after triggers change: %q", value)
						}
						return nil
					}),
					testutil.StandardSliceGraphChecks("fabric_slice.test"),
					testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
				),
			},
		},
	})
}
