package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestAccFabric_POA_Reboot provisions a slice, then runs a reboot POA against its
// sliver. In testmode MockAMHandler drives every POA to terminal Success with an empty
// info payload (A2), so this exercises the create + poll-to-terminal-success path
// end-to-end. It deliberately does NOT assert on `info` — that is always empty in
// testmode; info-shape assertions stay in the poa unit tests.
func TestAccFabric_POA_Reboot(t *testing.T) {
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{{
			Config: testutil.ProviderConfig(token) + sliceWithRebootPOAConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("fabric_poa.reboot", "state", "Success"),
				resource.TestCheckResourceAttrSet("fabric_poa.reboot", "poa_id"),
				// The POA runs against the slice's sliver, so the slice must be a
				// fully allocated VM.
				testutil.StandardSliceGraphChecks("fabric_slice.test"),
				testutil.CheckReservationMatchesSliverID("fabric_slice.test"),
			),
		}},
	})
}

func sliceWithRebootPOAConfig(name string) string {
	return fmt.Sprintf(`%s
resource "fabric_poa" "reboot" {
  sliver_id = fabric_slice.test.nodes["vm1"].sliver_id
  operation = "reboot"
}
`, testutil.BareVMConfig(name))
}
