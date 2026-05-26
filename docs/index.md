---
page_title: "FABRIC Provider"
subcategory: ""
description: |-
  The FABRIC provider manages FABRIC testbed slices from Terraform.
---

# FABRIC Provider

The FABRIC provider creates and manages FABRIC testbed slices using the FABRIC orchestrator API. A `fabric_slice` resource represents the complete slice topology: compute nodes, attached components, and network services are submitted as one GraphML model.

This design matches the orchestrator API. FABRIC accepts a complete topology document when creating or modifying a slice, so the provider intentionally models the slice as one Terraform resource instead of exposing separate node and network resources.

## Example Usage

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
  token            = var.fabric_token
  project_id       = var.fabric_project_id
  orchestrator_url = "https://orchestrator.fabric-testbed.net"

  project_tags = [
    "VM.NoLimitDisk",
    "Slice.Multisite",
  ]
}
```

## Authentication

The provider authenticates to the orchestrator with a FABRIC bearer token. The token is injected into generated orchestrator client requests through the client's `ContextAccessToken` authentication context.

The provider can read credentials from arguments or environment variables:

```shell
export FABRIC_TOKEN="<jwt from FABRIC portal>"
export FABRIC_PROJECT_ID="<project uuid>"
export FABRIC_ORCHESTRATOR_URL="https://orchestrator.fabric-testbed.net"
```

## Project Tags

FABRIC controls access to some resource classes and larger capacity requests with project tags. The provider validates those permissions before calling the orchestrator when `project_tags` is configured.

`project_tags` is a local declaration of tags available to the FABRIC project. The current implementation does not fetch project tags from the portal. If a configuration requests a gated resource and the tag is not present in `project_tags`, planning returns an attribute diagnostic naming the exact required tag.

Common examples:

```terraform
provider "fabric" {
  token      = var.fabric_token
  project_id = var.fabric_project_id

  project_tags = [
    "VM.NoLimitDisk",
    "Component.GPU",
    "Slice.Multisite",
  ]
}
```

## Provider Lifecycle

The provider uses these orchestrator operations:

- Create: `POST /slices/creates` with GraphML in the JSON request body.
- Read: `GET /slices/{slice_id}?graph_format=GRAPHML`.
- Update: `PUT /slices/modify/{slice_id}` with raw GraphML in the request body, followed by accept.
- Accept modify: `POST /slices/modify/{slice_id}/accept`.
- Renew: `POST /slices/renew/{slice_id}` for lifetime-only updates.
- Delete: `DELETE /slices/delete/{slice_id}`.
- Sliver outputs: `GET /slivers?slice_id=...`.
- Resources data source: `GET /resources`.

## Logging

The provider uses Terraform plugin logging (`tflog`). Token and SSH key values are marked sensitive and masked from logs. GraphML payload logging is trace-level and includes only size plus a short preview.

Useful Terraform logging variables:

```shell
TF_LOG=DEBUG terraform apply
TF_LOG_PROVIDER=TRACE terraform apply
```

## Acceptance Tests

Acceptance tests are gated and do not run unless all required variables are set:

```shell
export TF_ACC=1
export FABRIC_TOKEN="<jwt>"
export FABRIC_PROJECT_ID="<project uuid>"
export FABRIC_SSH_KEY="ssh-ed25519 AAAA..."
```

The included acceptance scope is intentionally narrow: bare VM, multisite VM, L2Bridge, update, import, and disappears flows. The live tests are designed around projects that have `Slice.Multisite` and `VM.NoLimitDisk`.

## Schema

### Optional

- `token` (String, Sensitive) FABRIC bearer token. May also be set with `FABRIC_TOKEN`.
- `orchestrator_url` (String) FABRIC orchestrator base URL. May also be set with `FABRIC_ORCHESTRATOR_URL`.
- `project_id` (String) FABRIC project UUID. May also be set with `FABRIC_PROJECT_ID`.
- `project_tags` (List of String) Permission tags available to the project.

## Resources

- [`fabric_slice`](resources/slice.md)

## Data Sources

- [`fabric_slice`](data-sources/slice.md)
- [`fabric_resources`](data-sources/resources.md)

## Guides

- [Usage examples](guides/examples.md)
- [Topology model](guides/topology-model.md)
- [FABRIC permission tags](guides/permissions.md)
- [Operational behavior](guides/operations.md)
