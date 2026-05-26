# Terraform Provider for FABRIC

This provider manages FABRIC testbed slices from Terraform. It is built around the FABRIC orchestrator model: a slice is submitted and modified as a complete GraphML topology, so the provider exposes one primary resource, `fabric_slice`, for the whole slice graph.

## Current Scope

Implemented provider surface:

- `fabric_slice` resource with create, read, update, delete, and import behavior.
- VM nodes with capacity, image, site, and component configuration.
- Catalog-backed component validation using `fabric-go-fim`.
- Network service blocks for `L2Bridge`, `L2STS`, `L2PTP`, `FABNetv4`, `FABNetv6`, `FABNetv4Ext`, `FABNetv6Ext`, and `PortMirror`.
- Permission tag diagnostics for gated VM, component, slice, and network features.
- Drift detection using semantic topology comparison.
- `fabric_slice` data source.
- `fabric_resources` data source.

Out of scope for this provider version:

- Facility port CRUD.
- Switch node CRUD.
- Explicit physical link CRUD.
- Subinterface CRUD.

## Quick Start

```terraform
terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = "~> 0.1"
    }
  }
}

provider "fabric" {
  token      = var.fabric_token
  project_id = var.fabric_project_id

  project_tags = [
    "VM.NoLimitDisk",
    "Slice.Multisite",
  ]
}

resource "fabric_slice" "example" {
  name    = "tf-example"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }
}
```

Environment variables are also supported:

```shell
export FABRIC_TOKEN="<jwt>"
export FABRIC_PROJECT_ID="<project uuid>"
export FABRIC_ORCHESTRATOR_URL="https://orchestrator.fabric-testbed.net"
```

## Documentation

- Provider configuration: [docs/index.md](docs/index.md)
- Slice resource: [docs/resources/slice.md](docs/resources/slice.md)
- Slice data source: [docs/data-sources/slice.md](docs/data-sources/slice.md)
- Resources data source: [docs/data-sources/resources.md](docs/data-sources/resources.md)
- Usage examples: [docs/guides/examples.md](docs/guides/examples.md)
- Topology model guide: [docs/guides/topology-model.md](docs/guides/topology-model.md)
- Permission tags guide: [docs/guides/permissions.md](docs/guides/permissions.md)
- Operational behavior guide: [docs/guides/operations.md](docs/guides/operations.md)

## Development

The workspace is expected to contain sibling modules:

```text
fabric-go-fim/
fabric-orchestrator-go-client/
terraform-provider-fabric/
```

From `terraform-provider-fabric`:

```shell
go build ./...
go test ./... -timeout 120s
```

Acceptance tests require live FABRIC credentials:

```shell
export TF_ACC=1
export FABRIC_TOKEN="<jwt>"
export FABRIC_PROJECT_ID="<project uuid>"
export FABRIC_SSH_KEY="ssh-ed25519 AAAA..."
go test ./internal/provider -run TestAccFabric -timeout 60m
```
