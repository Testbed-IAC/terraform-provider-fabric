# terraform-provider-fabric

[FABRIC](https://fabric-testbed.net/) is a national research testbed of interconnected compute, storage, and network resources across multiple sites. This provider manages FABRIC slices from Terraform: it builds a FIM topology from HCL, submits it to the FABRIC orchestrator, waits for a terminal provisioning state, and records computed runtime values such as slice IDs, sliver IDs, node states, and management IPs. It also reads advertised resources, decoded site capacity, sliver state, facility ports, and metrics, and runs perform-operational-action (POA) requests against provisioned slivers. It does not manage FABRIC projects, tokens, or user accounts, and it does not configure software inside a node beyond the boot, post-boot, and upload hooks the slice topology supports.

## Requirements

| Requirement | Version / Condition |
|---|---|
| Terraform | >= 1.0 (provider uses protocol 6 via terraform-plugin-framework) |
| Go | 1.24 (only to build from source) |
| FABRIC allocation | An active FABRIC project and a token from the [FABRIC portal](https://portal.fabric-testbed.net/) |

## Authentication

The provider authenticates with a FABRIC ID token (a bearer JWT) and resolves it from the first source that is set, in this order:

1. `token` in the provider block.
2. `token_file` in the provider block.
3. `FABRIC_TOKEN_LOCATION` environment variable (path to a token JSON file).
4. `~/.fabric/token.json`.
5. `~/work/fabric_config/id_token.json`.
6. `FABRIC_TOKEN` environment variable (a raw JWT).

Setting both `token` and `token_file` is an error. `orchestrator_url` defaults to `https://orchestrator.fabric-testbed.net` and `credmgr_url` defaults to `https://cm.fabric-testbed.net`; set them only to target a non-default deployment.

### Claims the provider reads

The provider decodes the JWT payload **without verifying its signature** — the orchestrator performs signature verification on every request. From the payload it reads exactly two claims:

- `projects[]` — only the **first** project entry is used. Its `name`, `uuid`, and `tags` drive the project ID sent to the orchestrator and the capability-tag checks below. A token scoped to a different project, or whose intended project is not first in the array, will be evaluated against the wrong project.
- `exp` — the expiry. All other claims (`sub`, `email`, `iss`, `aud`, and any portal-specific fields) are ignored.

### Static token vs. token file

The distinction controls refresh behavior:

- **Static token** (`token` attribute or `FABRIC_TOKEN`): used as-is and never refreshed. Once `exp` passes, the provider fails before any HTTP call with an instruction to fetch a fresh token. Suitable for CI where a short-lived token is injected per run.
- **Token file** (`token_file`, `FABRIC_TOKEN_LOCATION`, or a default path): the provider refreshes automatically. When the token is within 5 minutes of `exp`, it calls the credential manager `/credmgr/tokens/refresh` endpoint using the file's `refresh_token`, then rewrites the file in place with mode `0600`. A token file without a `refresh_token` cannot be refreshed and fails at expiry.

Token, SSH key, and refresh-token values are masked in plan output, state, and logs.

### Capability tags

FABRIC gates oversized requests and privileged resource types behind per-project permission tags carried in the token's first project. Before any orchestrator call, the provider compares the requested topology against the project tags and, for each missing tag, blocks the plan with an attribute error naming the tag and the project. Add the tag at the [FABRIC portal](https://portal.fabric-testbed.net/projects) (a project lead must do this), then request a fresh token.

A request triggers a tag only when it crosses the stated threshold; thresholds stack (for example, a 100-core node requires both `VM.NoLimitCPU` and `VM.NoLimit`).

| Tag | Required when | Consequence if missing |
|---|---|---|
| `VM.NoLimitCPU` | A node sets `cores` > 2 | Plan blocked on `node[i].cores` |
| `VM.NoLimitRAM` | A node sets `ram` > 8 (GB) | Plan blocked on `node[i].ram` |
| `VM.NoLimitDisk` | A node sets `disk` > 10 (GB) | Plan blocked on `node[i].disk` |
| `VM.NoLimit` | A node sets `cores` > 64, `ram` > 384 (GB), or `disk` > 1000 (GB) | Plan blocked on `node[i]` |
| `Slice.NoLimitLifetime` | `lifetime_hours` > 24 | Plan blocked on `lifetime_hours` |
| `Slice.Multisite` | Nodes span more than one `site` | Plan blocked on `node` |
| `Component.GPU` | A component sets `type = "GPU"` | Plan blocked on `node[i].component[j].type` |
| `Component.FPGA` | A component sets `type = "FPGA"` | Plan blocked on `node[i].component[j].type` |
| `Component.NVME` | A component sets `type = "NVME"` | Plan blocked on `node[i].component[j].type` |
| `Component.Storage` | A component sets `type = "Storage"` | Plan blocked on `node[i].component[j].type` |
| `Component.SmartNIC_ConnectX_5` | A `SmartNIC` component sets `model = "ConnectX-5"` | Plan blocked on `node[i].component[j].model` |
| `Component.SmartNIC_ConnectX_6` | A `SmartNIC` component sets `model = "ConnectX-6"` (also the fallback for an unrecognized SmartNIC model) | Plan blocked on `node[i].component[j].model` |
| `Component.SmartNIC_BlueField2_ConnectX_6` | A `SmartNIC` component sets `model = "BlueField-2-ConnectX-6"` | Plan blocked on `node[i].component[j].model` |
| `Component.SmartNIC_ConnectX_7_100` | A `SmartNIC` component sets `model = "ConnectX-7-100"` | Plan blocked on `node[i].component[j].model` |
| `Component.SmartNIC_ConnectX_7_400` | A `SmartNIC` component sets `model = "ConnectX-7-400"` | Plan blocked on `node[i].component[j].model` |
| `Net.FABNetv4Ext` | A network sets `type = "FABNetv4Ext"` | Plan blocked on `network[i].type` |
| `Net.FABNetv6Ext` | A network sets `type = "FABNetv6Ext"` | Plan blocked on `network[i].type` |
| `Net.PortMirroring` | A network sets `type = "PortMirror"` | Plan blocked on `network[i].type` |
| `Net.NoLimitBW` | A network sets `bandwidth` > 10000 | Plan blocked on `network[i].bandwidth` |

The SmartNIC tag is derived from the `model` string: hyphens are normalized to underscores and matched against the table above; an unrecognized model is treated as `ConnectX-6`. `SharedNIC` components and inferred OVS interfaces require no tag.

## Quick start

This config provisions one VM at a site from your project allocation. Set the token and SSH key with `TF_VAR_fabric_token` and `TF_VAR_ssh_public_key`, then `terraform apply`. It matches the minimal example in [`examples/resources/fabric_slice/basic.tf`](examples/resources/fabric_slice/basic.tf).

```hcl
terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = "~> 0.1"
    }
  }
}

variable "fabric_token" {
  description = "FABRIC ID token (JWT). Set via TF_VAR_fabric_token. Do not commit."
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key to install on slice nodes."
  type        = string
}

variable "site" {
  description = "FABRIC site name from your project allocation. Discover available sites with the fabric_sites data source."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Discover sites your allocation can use rather than hardcoding one.
data "fabric_sites" "available" {}

resource "fabric_slice" "quickstart" {
  name     = "quickstart-single-vm"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "vm1"
    site      = var.site
    image_ref = "default_rocky_9"
  }
}

output "management_ip" {
  value = fabric_slice.quickstart.nodes["vm1"].management_ip
}
```

## Resources

| Resource | Manages | Documentation |
|---|---|---|
| `fabric_slice` | A FABRIC slice: compute nodes, components, storage, facility ports, switches, and network services. Creates, reads, updates, imports, and deletes. | [docs/resources/slice.md](docs/resources/slice.md) |
| `fabric_poa` | One perform-operational-action request (reboot, CPU pinning, key add/remove, rescan, info) against a sliver. Action resource: replacing it re-runs the operation; deleting it only forgets state. | [docs/resources/poa.md](docs/resources/poa.md) |

## Data sources

| Data source | Reads | Documentation |
|---|---|---|
| `fabric_slice` | One slice by `slice_id`, `id`, or `name`, including computed runtime state. | [docs/data-sources/slice.md](docs/data-sources/slice.md) |
| `fabric_slivers` | Per-sliver state for a slice. | [docs/data-sources/slivers.md](docs/data-sources/slivers.md) |
| `fabric_sites` | Advertised resources decoded into typed site capacity, host, and component data, with include/exclude filters. | [docs/data-sources/sites.md](docs/data-sources/sites.md) |
| `fabric_resources` | Raw advertised resource models. | [docs/data-sources/resources.md](docs/data-sources/resources.md) |
| `fabric_facility_ports` | Advertised facility-port data. | [docs/data-sources/facility_ports.md](docs/data-sources/facility_ports.md) |
| `fabric_metrics` | Metrics overview results as JSON. | [docs/data-sources/metrics.md](docs/data-sources/metrics.md) |

## Building from source

```shell
go build ./...
go generate ./...   # regenerate docs/ from schema descriptions and examples/
```

Do not edit files under `docs/` by hand; they are generated from schema `Description`/`MarkdownDescription` fields, the `examples/` directory, and `templates/`. Building from source expects the sibling modules `fabric-go-fim/` and `fabric-orchestrator-go-client/` alongside this repository, as wired by the workspace `replace` directives.

## License

[MIT](/LICENSE)
