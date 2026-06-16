package provider_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// Validation acceptance tests. Each drives a deliberately invalid config through
// the real provider server and asserts the exact diagnostic. Every ExpectError
// regexp below is a substring of the message emitted by the named source:
//
//   - schema validators (OneOf/AtLeast/SizeAtLeast/Between/LengthAtMost) emit the
//     terraform-plugin-framework standard messages.
//   - CIDR/IP: internal/tfutil/validators.go (CIDRStringValidator, IPStringValidator)
//   - ssh_key/ssh_keys: internal/slice/slice_ssh_keys.go (validateSSHKeySource)
//   - bgp_key/host: internal/slice/label_schema.go (validateLabelsBlock, validateNodeHost)
//
// These steps fail at plan, so they create no infrastructure and need no
// CheckDestroy. They are gated on TF_ACC because the ModifyPlan-stage checks
// (bgp_key, host) run only after the provider configures against the stack.

// expectErrorStep is a single invalid-config plan step asserting errRE.
func expectErrorStep(token, body string, errRE *regexp.Regexp) resource.TestStep {
	return resource.TestStep{
		Config:      testutil.ProviderConfig(token) + body,
		ExpectError: errRE,
	}
}

func runValidationCase(t *testing.T, body string, errRE *regexp.Regexp) {
	t.Helper()
	testutil.SkipIfNoAcc(t)
	token := testutil.FullToken()
	name := testutil.UniqueName(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps:                    []resource.TestStep{expectErrorStep(token, fmt.Sprintf(body, name), errRE)},
	})
}

// bareNode is a minimal valid node body for embedding into invalid configs.
const bareNode = `
  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
  }
`

func TestAccFabric_Slice_SSHKeyConflict(t *testing.T) {
	// Both ssh_key and ssh_keys set -> validateSSHKeySource (slice_ssh_keys.go).
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_key  = "` + testutil.DummySSHKey + `"
  ssh_keys = ["` + testutil.DummySSHKey + `"]
` + bareNode + `}
`
	runValidationCase(t, body, regexp.MustCompile(`Configure exactly one of ssh_keys`))
}

func TestAccFabric_Slice_SSHKeysEmpty(t *testing.T) {
	// ssh_keys = [] -> listvalidator.SizeAtLeast(1).
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = []
` + bareNode + `}
`
	runValidationCase(t, body, regexp.MustCompile(`at least 1 element`))
}

func TestAccFabric_Slice_InvalidLifetime(t *testing.T) {
	// lifetime_hours = 0 -> int64validator.AtLeast(1).
	body := `
resource "fabric_slice" "test" {
  name           = %q
  ssh_keys       = ["` + testutil.DummySSHKey + `"]
  lifetime_hours = 0
` + bareNode + `}
`
	runValidationCase(t, body, regexp.MustCompile(`at least 1`))
}

func TestAccFabric_Slice_InvalidNetworkType(t *testing.T) {
	// network.type = "Bogus" -> stringvalidator.OneOf.
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]
` + bareNode + `
  network {
    name = "net1"
    type = "Bogus"
    interface {
      node = "vm1"
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`value must be one of`))
}

func TestAccFabric_Slice_InvalidComponentType(t *testing.T) {
	// component.type = "Bogus" -> stringvalidator.OneOf.
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    component {
      name = "c1"
      type = "Bogus"
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`value must be one of`))
}

func TestAccFabric_Slice_InvalidRouteCIDR(t *testing.T) {
	// route.subnet not CIDR -> tfutil.CIDRStringValidator.
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    route {
      subnet   = "not-a-cidr"
      next_hop = "10.0.0.1"
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`Invalid CIDR subnet`))
}

func TestAccFabric_Slice_InvalidRouteNextHop(t *testing.T) {
	// route.next_hop not an IP -> tfutil.IPStringValidator.
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    route {
      subnet   = "10.0.0.0/24"
      next_hop = "not-an-ip"
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`Invalid IP address`))
}

func TestAccFabric_Slice_NumaOutOfRange(t *testing.T) {
	// labels.numa = 99 -> int64validator.Between(-1, 7).
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    labels = {
      numa = 99
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`must be between -1 and 7`))
}

func TestAccFabric_Slice_BootScriptTooLong(t *testing.T) {
	// boot_script > 1024 chars -> stringvalidator.LengthAtMost(1024).
	tooLong := strings.Repeat("a", 1025)
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name        = "vm1"
    site        = "RENC"
    image_ref   = "default_rocky_9"
    boot_script = "` + tooLong + `"
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`at most 1024`))
}

func TestAccFabric_Slice_BGPKeyRequiresASN(t *testing.T) {
	// labels.bgp_key without labels.asn -> validateLabelsBlock (label_schema.go).
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    labels = {
      bgp_key = "secret-bgp-key"
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`bgp_key requires asn`))
}

func TestAccFabric_Slice_HostInstanceParentConflict(t *testing.T) {
	// node.host conflicts with labels.instance_parent -> validateNodeHost (label_schema.go).
	body := `
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = ["` + testutil.DummySSHKey + `"]

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
    host      = "renc-w1"
    labels = {
      instance_parent = "renc-w2"
    }
  }
}
`
	runValidationCase(t, body, regexp.MustCompile(`Conflicting FABRIC host labels`))
}
