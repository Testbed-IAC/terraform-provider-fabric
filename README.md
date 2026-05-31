# Terraform Provider for FABRIC

This provider manages FABRIC testbed slices from Terraform. `fabric_slice`
submits a complete FIM GraphML topology to the FABRIC orchestrator and refreshes
computed state such as sliver IDs, node states, and management IPs.

## Authentication

The provider resolves FABRIC credentials in this order:

1. `token` in the provider block.
2. `token_file` in the provider block.
3. `FABRIC_TOKEN_LOCATION`.
4. `~/.fabric/token.json` or `~/work/fabric_config/id_token.json` when present.
5. `FABRIC_TOKEN`.

`FABRIC_TOKEN` is a bearer JWT. `FABRIC_TOKEN_LOCATION` points at a FABRIC portal
token JSON file. Set exactly one of `token` and `token_file` in configuration.

```hcl
provider "fabric" {
  orchestrator_url = "https://orchestrator.fabric-testbed.net"
  credmgr_url      = "https://cm.fabric-testbed.net"
}
```

`orchestrator_url` and `credmgr_url` are optional and default to the public
FABRIC services. The provider derives project ID and permission tags from the
token claims and validates gated features during planning.

## Resources and Data Sources

- `fabric_slice` creates, updates, imports, reads, and deletes slices.
- `fabric_poa` runs a perform-operational-action request against a sliver.
- `fabric_slice` data source reads a slice by ID or name.
- `fabric_resources` returns raw advertised resource models, with date and site filters.
- `fabric_sites` decodes verified advertised site fields.
- `fabric_facility_ports` decodes verified advertised facility-port fields.
- `fabric_slivers` exposes per-sliver state for a slice.
- `fabric_metrics` returns metrics overview results as JSON.

## Slice Configuration

Use `ssh_keys` for one or more public keys:

```hcl
resource "fabric_slice" "example" {
  name     = "tf-example"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "vm1"
    site = "RENC"
  }
}
```

`ssh_key` is deprecated but remains as a single-key compatibility alias. Configure
exactly one of `ssh_keys` or `ssh_key`. SSH key material is not retained in state
after apply, so import and refresh cannot reconstruct it.

Changing `name`, `ssh_key`, `ssh_keys`, or `ssh_key_version` forces replacement.
`fabric_poa` is an action resource: changing `sliver_id`, `operation`,
`vcpu_cpu_map`, `node_set`, `bdf`, `keys`, or `triggers` replaces the resource and
re-runs the operation. Deleting a POA resource only forgets Terraform state; the
FABRIC operation cannot be undone.

## Permission Tags

The provider validates FABRIC permission tags before API calls. Diagnostics name
the required tag, such as `Slice.Multisite`, `Component.GPU`,
`Component.FPGA`, `Component.NVME`, `Net.PortMirroring`, or extended FABNet
permissions. If a project lacks a tag, remove the gated feature or request the
permission in FABRIC.

## Import Limitations

`terraform import fabric_slice.example <slice-id>` records the slice ID and reads
orchestrator-computed fields. It cannot reconstruct the original HCL topology,
SSH public key inputs, write-only key material, comments, or local variables.
Write matching `node` and `network` blocks by hand before planning.

## Drift Reconciliation

FABRIC assigns runtime values such as IPs, MACs, VLANs, sliver IDs, management
IPs, reservations, and allocation records. Those fields are treated as computed
state and do not produce drift warnings.

Configuration-owned changes such as node counts, sites, types, capacities, and
labels are reported as topology drift during refresh. FABRIC does not support
full bidirectional reconciliation, so resolve drift by updating Terraform
configuration or replacing/modifying the slice.

## Examples

Runnable examples live under `examples/` for labels, multiple SSH keys, and typed
site discovery. Some files are intentionally marked illustrative where the full
schema surface is not present in this checkout.

## Development

The workspace is expected to contain sibling modules:

```text
fabric-go-fim/
fabric-orchestrator-go-client/
terraform-provider-fabric/
```

Useful commands:

```shell
go build ./...
go vet ./...
go test ./... -timeout 120s
go test ./... -race
golangci-lint run ./...
make docs
make test
```

Acceptance tests use live FABRIC credentials:

```shell
export TF_ACC=1
export FABRIC_TOKEN="<jwt>"
export FABRIC_SSH_KEY="ssh-ed25519 AAAA..."
make testacc
```
