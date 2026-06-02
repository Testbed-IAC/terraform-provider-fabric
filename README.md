# Terraform Provider for FABRIC

This provider manages experiment infrastructure on the
[FABRIC testbed](https://fabric-testbed.net/) from Terraform. It creates and
updates FABRIC slices, inspects advertised resources and sliver state, and runs
perform-operational-action requests against provisioned slivers.

`fabric_slice` builds a FIM topology, submits it to the FABRIC orchestrator, and
refreshes computed runtime state such as slice IDs, graph IDs, sliver IDs, node
states, and management IPs.

## Authentication

The provider resolves FABRIC credentials in this order:

1. `token` in the provider block.
2. `token_file` in the provider block.
3. `FABRIC_TOKEN_LOCATION`.
4. `~/.fabric/token.json`.
5. `~/work/fabric_config/id_token.json`.
6. `FABRIC_TOKEN`.

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

Resources:

- `fabric_slice` creates, updates, imports, reads, and deletes slices.
- `fabric_poa` runs a perform-operational-action request against a sliver.

Data sources:

- `fabric_slice` reads a slice by `slice_id`, `id`, or `name`.
- `fabric_resources` returns raw advertised resource models.
- `fabric_sites` decodes advertised site capacity, host, and component data.
- `fabric_facility_ports` decodes advertised facility-port data.
- `fabric_slivers` exposes per-sliver state for a slice.
- `fabric_metrics` returns metrics overview results as JSON.

## Slice Configuration

Use `ssh_keys` for one or more public keys:

```hcl
variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}

resource "fabric_slice" "single_vm" {
  name     = "ren-dev-single-vm"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "login"
    site = "RENC"
  }
}
```

`ssh_key` is deprecated but remains as a single-key compatibility alias.
Configure exactly one of `ssh_keys` or `ssh_key`. SSH key material is masked in
plan output and state.

Changing `name`, `ssh_key`, `ssh_keys`, or `ssh_key_version` forces replacement.
`fabric_poa` is an action resource: changing `sliver_id`, `operation`,
`vcpu_cpu_map`, `node_set`, `bdf`, `keys`, or `triggers` replaces the resource
and re-runs the operation. Deleting a POA resource only forgets Terraform state;
the FABRIC operation cannot be undone.

## Examples

Runnable examples live under `examples/` and are embedded into generated
Registry docs by `tfplugindocs`:

```text
examples/
├── provider/provider.tf
├── resources/
│   ├── fabric_slice/resource.tf
│   ├── fabric_slice/import.sh
│   └── fabric_poa/resource.tf
└── data-sources/
    ├── fabric_facility_ports/data-source.tf
    ├── fabric_metrics/data-source.tf
    ├── fabric_resources/data-source.tf
    ├── fabric_sites/data-source.tf
    ├── fabric_slice/data-source.tf
    └── fabric_slivers/data-source.tf
```

Run `terraform fmt -recursive examples` after editing examples.

## Documentation

Provider documentation is generated from schema descriptions and examples using
`tfplugindocs`. Do not edit files under `docs/` by hand; update schema
`Description` and `MarkdownDescription` fields, examples, or templates instead.

```shell
go generate ./...
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-name fabric
```

The provider index page is customized with `templates/index.md.tmpl`. Resource
and data source pages use the default generated templates.

## Permission Tags

The provider validates FABRIC permission tags before API calls. Diagnostics name
the required tag, such as `Slice.Multisite`, `Component.GPU`,
`Component.FPGA`, `Component.NVME`, `Net.PortMirroring`, or extended FABNet
permissions. If a project lacks a tag, remove the gated feature or request the
permission in FABRIC.

## Import Limitations

`terraform import fabric_slice.single_vm <slice-id>` records the slice ID and
reads orchestrator-computed fields. It cannot reconstruct the original HCL
topology, SSH public key inputs, write-only key material, comments, or local
variables. Write matching topology blocks by hand before planning.

## Drift Reconciliation

FABRIC assigns runtime values such as IPs, MACs, VLANs, sliver IDs, management
IPs, reservations, and allocation records. Those fields are treated as computed
state and do not produce drift warnings.

Configuration-owned changes such as node counts, sites, types, capacities, and
labels are reported as topology drift during refresh. FABRIC does not support
full bidirectional reconciliation, so resolve drift by updating Terraform
configuration or replacing/modifying the slice.

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
