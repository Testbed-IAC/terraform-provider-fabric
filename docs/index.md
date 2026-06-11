---
page_title: "Provider: FABRIC"
description: |-
  Terraform provider for creating and inspecting FABRIC testbed slices, resources, slivers, and operational actions.
---

# FABRIC Provider

The FABRIC provider manages experiment infrastructure on the [FABRIC testbed](https://fabric-testbed.net/). It can create slices, inspect advertised site and facility-port capacity, read sliver runtime state, and run perform-operational-action requests against provisioned slivers.

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

variable "fabric_token" {
  description = "FABRIC ID token (JWT). Set via TF_VAR_fabric_token or a .tfvars file. Do not commit."
  type        = string
  sensitive   = true
}

provider "fabric" {
  token = var.fabric_token
}

# Omit token to resolve credentials from FABRIC_TOKEN_LOCATION, a token file at
# ~/.fabric/token.json or ~/work/fabric_config/id_token.json, or FABRIC_TOKEN. A
# token file refreshes automatically using its refresh_token; a static token does
# not. orchestrator_url and credmgr_url default to the production deployment.
```

## Authentication

The provider resolves credentials in this order:

1. `token` in the provider block.
2. `token_file` in the provider block.
3. `FABRIC_TOKEN_LOCATION`.
4. `~/.fabric/token.json`.
5. `~/work/fabric_config/id_token.json`.
6. `FABRIC_TOKEN`.

Set exactly one of `token` and `token_file` in provider configuration. In normal usage, prefer `FABRIC_TOKEN_LOCATION` or a token file so the provider can refresh credentials using the file's refresh token.

## Argument Reference

* `token` - (Optional, Sensitive) FABRIC bearer JWT used for authentication. This value is masked in plan output and state. May also be set with `FABRIC_TOKEN`.
* `token_file` - (Optional) Path to a FABRIC portal token JSON file. May also be set with `FABRIC_TOKEN_LOCATION`.
* `orchestrator_url` - (Optional) FABRIC orchestrator base URL. Defaults to `https://orchestrator.fabric-testbed.net`.
* `credmgr_url` - (Optional) FABRIC credential manager base URL. Defaults to `https://cm.fabric-testbed.net`.

## Environment Variables

| Argument | Environment Variable |
|---|---|
| `token` | `FABRIC_TOKEN` |
| `token_file` | `FABRIC_TOKEN_LOCATION` |
