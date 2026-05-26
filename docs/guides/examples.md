---
page_title: "Usage Examples"
subcategory: "Guides"
description: |-
  End-to-end FABRIC provider usage examples.
---

# Usage Examples

This guide collects end-to-end patterns for the provider as implemented in this repository. Each example uses the current `fabric_slice` single-resource topology model and the current schema fields.

## Shared Variables

Most examples use the same variable set:

```terraform
variable "fabric_token" {
  type      = string
  sensitive = true
  default   = null
}

variable "fabric_project_id" {
  type    = string
  default = null
}

variable "fabric_ssh_key" {
  type      = string
  sensitive = true
}

variable "fabric_orchestrator_url" {
  type    = string
  default = "https://orchestrator.fabric-testbed.net"
}
```

You can also omit provider arguments and use environment variables:

```shell
export FABRIC_TOKEN="<jwt>"
export FABRIC_PROJECT_ID="<project uuid>"
export FABRIC_SSH_KEY="ssh-ed25519 AAAA..."
export FABRIC_ORCHESTRATOR_URL="https://orchestrator.fabric-testbed.net"
```

## Provider With Explicit Permission Tags

Use `project_tags` when your configuration requests gated resources. The provider checks this list locally before calling the orchestrator.

```terraform
provider "fabric" {
  token            = var.fabric_token
  project_id       = var.fabric_project_id
  orchestrator_url = var.fabric_orchestrator_url

  project_tags = [
    "VM.NoLimitCPU",
    "VM.NoLimitRAM",
    "VM.NoLimitDisk",
    "Slice.Multisite",
    "Component.SmartNIC_ConnectX_6",
    "Net.NoLimitBW",
  ]
}
```

## Minimal Single-VM Slice

This is the smallest useful slice. The provider supplies default VM capacity and image values in the topology builder.

```terraform
resource "fabric_slice" "single_vm" {
  name    = "tf-single-vm"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }
}

output "vm1_management_ip" {
  value = fabric_slice.single_vm.nodes["vm1"].management_ip
}

output "slice_state" {
  value = fabric_slice.single_vm.state
}
```

Builder defaults:

| Field | Value |
| --- | --- |
| `cores` | `2` |
| `ram` | `8` |
| `disk` | `10` |
| `image_ref` | `default_rocky_9` |
| `image_type` | `qcow2` |

## Larger VM With Renewal-Friendly Lifetime

Changing only `lifetime_hours` uses the provider's renew path instead of a topology modify.

```terraform
resource "fabric_slice" "larger_vm" {
  name           = "tf-larger-vm"
  ssh_key        = var.fabric_ssh_key
  lifetime_hours = 24

  node {
    name  = "vm1"
    site  = "RENC"
    cores = 4
    ram   = 16
    disk  = 50
  }

  timeouts {
    create = "45m"
    update = "45m"
    delete = "30m"
  }
}
```

Required provider tags for this example:

- `VM.NoLimitCPU`
- `VM.NoLimitRAM`
- `VM.NoLimitDisk`

If you later change only `lifetime_hours`, the provider calls `RenewSlice`. If you change node capacity or topology, the provider calls modify and then accept.

## L2Bridge With Shared NICs

This creates two VMs at the same site, each with a shared NIC, connected by an `L2Bridge`.

```terraform
resource "fabric_slice" "l2bridge" {
  name    = "tf-l2bridge"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name = "vm2"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "lan1"
    type = "L2Bridge"

    interface {
      node      = "vm1"
      component = "nic1"
      port      = 0
    }

    interface {
      node      = "vm2"
      component = "nic1"
      port      = 0
    }
  }
}
```

The provider resolves `component = "nic1"` to the generated FIM component under each VM. `port = 0` selects the first component interface.

## Multisite L2STS

This pattern spans sites and requires `Slice.Multisite`.

```terraform
resource "fabric_slice" "multisite_l2sts" {
  name    = "tf-multisite-l2sts"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm-renc"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name = "vm-uky"
    site = "UKY"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "l2sts1"
    type = "L2STS"

    interface {
      node      = "vm-renc"
      component = "nic1"
      port      = 0
    }

    interface {
      node      = "vm-uky"
      component = "nic1"
      port      = 0
    }
  }
}
```

## SmartNIC With FABlib Alias

`fablib_name` resolves through the embedded catalog and takes precedence over `type` and `model`.

```terraform
resource "fabric_slice" "smartnic" {
  name    = "tf-smartnic"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"

    component {
      name        = "snic"
      fablib_name = "NIC_ConnectX_6"
    }
  }
}
```

Required provider tag:

- `Component.SmartNIC_ConnectX_6`

## L2PTP With SmartNIC Dedicated Ports

SmartNIC and FPGA components expose dedicated ports. This example connects the first dedicated port on each VM with `L2PTP`.

```terraform
resource "fabric_slice" "l2ptp" {
  name    = "tf-l2ptp"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm-renc"
    site = "RENC"

    component {
      name  = "snic"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name = "vm-uky"
    site = "UKY"

    component {
      name  = "snic"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "ptp1"
    type = "L2PTP"

    interface {
      node      = "vm-renc"
      component = "snic"
      port      = 0
    }

    interface {
      node      = "vm-uky"
      component = "snic"
      port      = 0
    }
  }
}
```

Required provider tags:

- `Slice.Multisite`
- `Component.SmartNIC_ConnectX_6`

## FABNetv4

The schema supports routed FABNet service types. The current resource schema does not expose gateway customization fields, so this example creates the basic service shape.

```terraform
resource "fabric_slice" "fabnetv4" {
  name    = "tf-fabnetv4"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name = "vm2"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "v4net"
    type = "FABNetv4"

    interface {
      node      = "vm1"
      component = "nic1"
      port      = 0
    }

    interface {
      node      = "vm2"
      component = "nic1"
      port      = 0
    }
  }
}
```

For `FABNetv4Ext`, add the provider tag `Net.FABNetv4Ext`. For `FABNetv6Ext`, add `Net.FABNetv6Ext`.

## PortMirror Shape

`PortMirror` requires a source interface name, destination interface, and mirror direction. The source name must match an interface name in the built topology.

```terraform
resource "fabric_slice" "port_mirror" {
  name    = "tf-port-mirror"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm-source"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name = "vm-dest"
    site = "RENC"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name             = "mirror1"
    type             = "PortMirror"
    mirror_from      = "nic1-p1"
    mirror_direction = "Both"

    interface {
      node      = "vm-dest"
      component = "nic1"
      port      = 0
    }
  }
}
```

Required provider tag:

- `Net.PortMirroring`

## Data Source Lookup After Creating a Slice

```terraform
resource "fabric_slice" "example" {
  name    = "tf-lookup-example"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }
}

data "fabric_slice" "created" {
  slice_id = fabric_slice.example.slice_id
}

output "created_state" {
  value = data.fabric_slice.created.state
}
```

## Resources Data Source

```terraform
data "fabric_resources" "current" {
  level         = 1
  force_refresh = false
}

output "resources_model" {
  value = data.fabric_resources.current.model
}
```

## Importing an Existing Slice

First write configuration matching the existing slice topology:

```terraform
resource "fabric_slice" "imported" {
  name    = "existing-slice-name"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }
}
```

Then import with the slice id:

```shell
terraform import fabric_slice.imported <slice_id>
terraform plan
```

If the HCL topology does not match the imported slice, the next plan will show changes to reconcile FABRIC back to Terraform.

## Drift Investigation Workflow

When read detects drift, Terraform emits warnings. A practical workflow is:

```shell
TF_LOG_PROVIDER=DEBUG terraform plan
```

Then inspect:

- Drift warnings in Terraform output.
- `fabric_slice.<name>.state`.
- `fabric_slice.<name>.nodes` for per-node sliver state and error messages.

Because drift is a warning, not a hard failure, Terraform can still produce a plan.
