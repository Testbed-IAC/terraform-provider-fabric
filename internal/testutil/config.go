package testutil

import (
	"fmt"
	"strings"
)

// ProviderConfig returns an HCL `provider "fabric"` block with the token inlined and
// the orchestrator URL pointed at testmode. Concatenate with a resource/data block.
func ProviderConfig(token string) string {
	return fmt.Sprintf(`
provider "fabric" {
  token            = %q
  orchestrator_url = %q
}
`, token, OrchestratorURL())
}

// VMOption mutates a node's optional fields in the HCL builders.
type VMOption func(*vmConfig)

type vmConfig struct {
	site             string
	image            string
	cores, ram, disk int
	lifetimeHours    int
}

func defaultVM() vmConfig {
	return vmConfig{site: TestSite, image: "default_rocky_9"}
}

// WithLifetime sets the slice lifetime_hours. Changing only this between applies
// exercises the provider's in-place lease-renew path (not topology modify).
func WithLifetime(hours int) VMOption { return func(c *vmConfig) { c.lifetimeHours = hours } }

// WithImage overrides the node image_ref.
func WithImage(image string) VMOption { return func(c *vmConfig) { c.image = image } }

// WithCapacity sets explicit cores/ram/disk. A zero value is omitted from the config
// (provider applies its own defaults).
func WithCapacity(cores, ram, disk int) VMOption {
	return func(c *vmConfig) { c.cores, c.ram, c.disk = cores, ram, disk }
}

func (c vmConfig) nodeBlock(name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  node {\n    name = %q\n    site = %q\n", name, c.site)
	if c.image != "" {
		fmt.Fprintf(&b, "    image_ref = %q\n", c.image)
	}
	if c.cores > 0 {
		fmt.Fprintf(&b, "    cores = %d\n", c.cores)
	}
	if c.ram > 0 {
		fmt.Fprintf(&b, "    ram = %d\n", c.ram)
	}
	if c.disk > 0 {
		fmt.Fprintf(&b, "    disk = %d\n", c.disk)
	}
	b.WriteString("  }\n")
	return b.String()
}

// BareVMConfig returns a single-VM slice resource named "test" (node "vm1").
func BareVMConfig(name string, opts ...VMOption) string {
	cfg := defaultVM()
	for _, o := range opts {
		o(&cfg)
	}
	lifetimeLine := ""
	if cfg.lifetimeHours > 0 {
		lifetimeLine = fmt.Sprintf("  lifetime_hours = %d\n", cfg.lifetimeHours)
	}
	return fmt.Sprintf(`
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = [%q]
%s
%s}
`, name, DummySSHKey, lifetimeLine, cfg.nodeBlock("vm1"))
}

// VMWithComponentConfig returns a single-VM slice with one attached component.
// componentType is one of GPU/SmartNIC/SharedNIC/FPGA/NVME/Storage; model is the
// catalog model string (e.g. "RTX6000", "ConnectX-6", "P4510").
func VMWithComponentConfig(name, componentType, model string, opts ...VMOption) string {
	cfg := defaultVM()
	for _, o := range opts {
		o(&cfg)
	}
	node := strings.TrimRight(cfg.nodeBlock("vm1"), "\n")
	// Re-open the node block to insert the component before its closing brace.
	node = strings.TrimSuffix(node, "  }")
	comp := fmt.Sprintf("    component {\n      name  = \"c1\"\n      type  = %q\n      model = %q\n    }\n  }\n", componentType, model)
	return fmt.Sprintf(`
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = [%q]

%s%s}
`, name, DummySSHKey, node, comp)
}

// TwoSiteNodesConfig returns a slice with two bare VMs at two sites and NO network
// service. This is the genuine multi-site placement test: both nodes are node-only and
// broker-satisfiable, so the slice reaches StableOK (a network service would hit the
// testmode network-reservation hang). Requires the §4f UKY site AM for siteB=UKY.
func TwoSiteNodesConfig(name, siteA, siteB string) string {
	if siteA == "" {
		siteA = TestSite
	}
	if siteB == "" {
		siteB = TestSiteUKY
	}
	node := func(nodeName, site string) string {
		return fmt.Sprintf("  node {\n    name      = %q\n    site      = %q\n    image_ref = \"default_rocky_9\"\n  }\n", nodeName, site)
	}
	return fmt.Sprintf(`
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = [%q]

%s%s}
`, name, DummySSHKey, node("vm1", siteA), node("vm2", siteB))
}

// TwoNodeNetworkConfig returns a slice with two VMs (each with a SharedNIC) joined by a
// network service of the given type. When netType is empty, the type is omitted so the
// provider infers it. siteA/siteB let multi-site tests place nodes at RENC and UKY.
func TwoNodeNetworkConfig(name, netType, siteA, siteB string) string {
	if siteA == "" {
		siteA = TestSite
	}
	if siteB == "" {
		siteB = TestSite
	}
	typeLine := ""
	if netType != "" {
		typeLine = fmt.Sprintf("    type = %q\n", netType)
	}
	node := func(nodeName, site string) string {
		return fmt.Sprintf(`  node {
    name      = %q
    site      = %q
    image_ref = "default_rocky_9"
    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }
`, nodeName, site)
	}
	return fmt.Sprintf(`
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = [%q]

%s%s  network {
    name = "net1"
%s    interface {
      node      = "vm1"
      component = "nic1"
    }
    interface {
      node      = "vm2"
      component = "nic1"
    }
  }
}
`, name, DummySSHKey, node("vm1", siteA), node("vm2", siteB), typeLine)
}
