package testutil

import "fmt"

// This file holds HCL builders for the new acceptance tests. They return only the
// resource/data blocks; callers prepend ProviderConfig. Site/image values come
// from constants.go (TestSite) and the provider's documented defaults.

// NodeUserDataConfig is a single bare VM exercising the node user-data fields:
// boot_script (<=1024 chars), post_boot_execute, and post_update.
func NodeUserDataConfig(name string) string {
	return fmt.Sprintf(`
resource "fabric_slice" "test" {
  name     = %q
  ssh_keys = [%q]

  node {
    name      = "vm1"
    site      = %q
    image_ref = "default_rocky_9"

    boot_script       = "#!/bin/bash\necho fabric-acc-boot"
    post_boot_execute = ["echo post-boot-one", "echo post-boot-two"]
    post_update       = ["echo post-update-one"]
  }
}
`, name, DummySSHKey, TestSite)
}

// LeaseStartConfig is a single bare VM with an explicit lease_start_time. The
// caller supplies a time string the FabricTimeValidator accepts (RFC3339 or
// FABRIC format).
func LeaseStartConfig(name, leaseStart string) string {
	return fmt.Sprintf(`
resource "fabric_slice" "test" {
  name             = %q
  ssh_keys         = [%q]
  lease_start_time = %q

  node {
    name      = "vm1"
    site      = %q
    image_ref = "default_rocky_9"
  }
}
`, name, DummySSHKey, leaseStart, TestSite)
}

// SliceWithSliceLookupsConfig provisions a bare VM and looks it up through the
// fabric_slice data source by slice_id and by id (the two non-name lookup keys).
func SliceWithSliceLookupsConfig(name string) string {
	return fmt.Sprintf(`%s
data "fabric_slice" "by_slice_id" {
  slice_id = fabric_slice.test.slice_id
}

data "fabric_slice" "by_id" {
  id = fabric_slice.test.id
}
`, BareVMConfig(name))
}

// SliceWithSliversDataSourceConfig provisions a bare VM and reads its slivers
// through the fabric_slivers data source.
func SliceWithSliversDataSourceConfig(name string) string {
	return fmt.Sprintf(`%s
data "fabric_slivers" "all" {
  slice_id = fabric_slice.test.slice_id
}
`, BareVMConfig(name))
}

// MetricsDataSourceConfig reads the orchestrator metrics overview. The testmode
// orchestrator serves GET /metrics/overview from its Postgres slice counts.
func MetricsDataSourceConfig() string {
	return `
data "fabric_metrics" "overview" {
}
`
}

// ResourcesOptionsDataSourceConfig reads available resources at level 2 with
// force_refresh, exercising the resources data source options.
func ResourcesOptionsDataSourceConfig() string {
	return `
data "fabric_resources" "lvl2" {
  level         = 2
  force_refresh = true
}
`
}

// SitesDataSourceConfig reads the typed sites data source. Against the testmode
// broker CBM, catalog.DecodeAdvertised returns zero sites (a documented decoder
// gap), so callers assert graceful empty decoding rather than populated sites.
func SitesDataSourceConfig() string {
	return `
data "fabric_sites" "all" {
}
`
}

// FacilityPortsDataSourceConfig reads the typed facility-ports data source. Unlike
// fabric_sites (whose aggregating Type="Site" node is absent from the testmode CBM),
// the network AM's Network-ad.graphml advertises FacilityPort nodes, so this decodes
// to a non-empty list (typically 3 entries).
func FacilityPortsDataSourceConfig() string {
	return `
data "fabric_facility_ports" "all" {
}
`
}
