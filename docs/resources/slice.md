---
page_title: "fabric_slice Resource"
subcategory: ""
description: |-
  Manages a complete FABRIC slice topology.
---

# fabric_slice

`fabric_slice` manages a complete FABRIC slice as a single Terraform resource. The resource builds GraphML from Terraform configuration using `fabric-go-fim`, submits it to the orchestrator, polls the slice state, and refreshes node outputs from the returned topology and sliver list.

The resource intentionally owns the whole topology. FABRIC modify operations are whole-slice operations, so node, component, interface, and network changes are applied by submitting a new complete graph and accepting the modify result.

## Example Usage

For larger end-to-end configurations, see the [usage examples guide](../guides/examples.md).

### Bare VM

```terraform
resource "fabric_slice" "bare_vm" {
  name           = "tf-bare-vm"
  ssh_key        = var.fabric_ssh_key
  lifetime_hours = 24

  node {
    name = "vm1"
    site = "RENC"
  }
}

output "vm1_management_ip" {
  value = fabric_slice.bare_vm.nodes["vm1"].management_ip
}
```

### VM With Explicit Capacity

```terraform
resource "fabric_slice" "larger_vm" {
  name    = "tf-larger-vm"
  ssh_key = var.fabric_ssh_key

  node {
    name  = "vm1"
    site  = "RENC"
    cores = 4
    ram   = 16
    disk  = 50
  }
}
```

Requests above the default VM limits require permission tags such as `VM.NoLimitCPU`, `VM.NoLimitRAM`, and `VM.NoLimitDisk` in provider `project_tags`.

### VM With Shared NIC

```terraform
resource "fabric_slice" "shared_nic" {
  name    = "tf-shared-nic"
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
}
```

### L2Bridge

```terraform
resource "fabric_slice" "lan" {
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

### Multisite L2STS

```terraform
resource "fabric_slice" "l2sts" {
  name    = "tf-l2sts"
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
    name = "lan1"
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

Multisite topologies require the `Slice.Multisite` project tag.

## Lifecycle Behavior

### Create

Create builds GraphML with `topology.NewWithID(topology.DeriveGraphID(name))` and calls the orchestrator create endpoint. The resource writes partial state as soon as the create response returns a slice id, then polls until `StableOK` or a terminal failure state.

Default VM build values in this provider:

- `cores`: `2`
- `ram`: `8`
- `disk`: `10`
- `image_ref`: `default_rocky_9`
- `image_type`: `qcow2`

### Read

Read fetches the returned GraphML and refreshes computed fields. If the slice is gone, `Dead`, `Closing`, or `ClosingError`, Terraform state is removed.

If a slice is in `StableError` or `ModifyError`, the resource preserves state and emits a warning with the orchestrator notice when available.

If a slice is stuck in `ModifyOK`, read attempts accept recovery.

### Update

Update validates permissions and rebuilds the desired topology.

If only `lifetime_hours` changes and the topology is otherwise equivalent, the provider uses the renew endpoint instead of modify.

Topology changes are sent through the modify endpoint. The provider polls for `ModifyOK`, `ModifyError`, or `StableOK`, then calls accept. Accept is called even after `ModifyError` so the orchestrator can prune failed resources.

### Delete

Delete calls the slice delete endpoint and polls until `Dead` or `ClosingError`. A 404 is treated as success because the slice is already gone.

## Drift Detection

Read compares the desired topology rebuilt from Terraform state with the GraphML returned by the orchestrator. Drift is surfaced as Terraform warnings, not hard errors.

This lets `terraform plan` finish even when FABRIC state changed outside Terraform, for example from portal edits, sliver expiration, or hardware failure. The warning describes the semantic topology difference. A subsequent apply reconciles the topology back to the Terraform configuration.

## Import

Import uses the FABRIC slice id:

```shell
terraform import fabric_slice.example 11111111-2222-3333-4444-555555555555
```

The import implementation sets both `id` and `slice_id`. A refresh then fills computed state from the orchestrator. To avoid a replacement plan after import, the Terraform configuration must describe the same topology as the imported slice.

## Schema

### Required

- `name` (String) Slice name. Changing this forces replacement because graph identity is derived from the name.
- `ssh_key` (String, Sensitive) Public SSH key sent to the orchestrator for VM login.
- `node` (Block List) One or more VM nodes.

### Optional

- `lifetime_hours` (Number) Requested lifetime in hours. Defaults to `24`. Updating only this field uses slice renew.
- `lease_start_time` (String) Optional RFC 3339 lease start time. Changing this forces replacement.
- `network` (Block List) Network services connecting node interfaces.
- `timeouts` (Block) Operation timeouts for create, read, update, and delete.

### Read-Only

- `id` (String) Terraform id. Same value as `slice_id`.
- `slice_id` (String) Orchestrator slice UUID.
- `graph_id` (String) Orchestrator graph id.
- `state` (String) Orchestrator slice state.
- `lease_end_time` (String) Lease end time returned by the orchestrator.
- `nodes` (Map) Per-node computed outputs keyed by node name.

## Nested Schema for `node`

### Required

- `name` (String) Node name. Changing this forces replacement.
- `site` (String) FABRIC site code such as `RENC` or `UKY`. Changing this forces replacement.

### Optional

- `instance_type` (String) FABRIC instance flavor hint validated against the embedded catalog when set.
- `image_ref` (String) Image reference. Defaults to `default_rocky_9` when omitted.
- `image_type` (String) Image type. Defaults to `qcow2` when omitted.
- `cores` (Number) VM cores. Defaults to `2` in the topology builder.
- `ram` (Number) VM RAM in GB. Defaults to `8`.
- `disk` (Number) VM disk in GB. Defaults to `10`.
- `component` (Attribute List) Components attached to the VM.

## Nested Schema for `node.component`

### Required

- `name` (String) Component name within the node.

### Optional

- `type` (String) Component type. Valid values: `GPU`, `SmartNIC`, `SharedNIC`, `FPGA`, `NVME`, `Storage`.
- `model` (String) Catalog model such as `ConnectX-6`, `RTX6000`, or `P4510`.
- `fablib_name` (String) FABlib alias resolved through the embedded catalog. When set, it takes precedence over `type` and `model`.

The provider validates component type and model against `fabric-go-fim/pkg/catalog`.

## Nested Schema for `network`

### Required

- `name` (String) Network service name.
- `type` (String) Network service type. Valid values: `L2Bridge`, `L2STS`, `L2PTP`, `FABNetv4`, `FABNetv6`, `FABNetv4Ext`, `FABNetv6Ext`, `PortMirror`.

### Optional

- `bandwidth` (Number) Requested network bandwidth. Values above `10000` require `Net.NoLimitBW`.
- `mirror_from` (String) Source interface name for `PortMirror`.
- `mirror_direction` (String) Mirror direction for `PortMirror`. Valid values: `Both`, `RX_Only`, `TX_Only`.
- `interface` (Block List) Component or node interfaces connected to the network.

## Nested Schema for `network.interface`

### Required

- `node` (String) Node name containing the interface.

### Optional

- `component` (String) Component name. The provider accepts either the child name (`nic1`) or final FIM component name (`vm1-nic1`).
- `port` (Number) Zero-based interface index. Defaults to `0`.
- `name` (String) Interface name. Used when selecting by name instead of component and port.

When `component` is set, the provider selects `component.Interfaces()[port]`. When `component` is omitted and `name` is set, it searches the node interface list by name. Otherwise it selects the node interface list by `port`.

## Nested Schema for `nodes`

The computed `nodes` map is keyed by Terraform node name.

Each value contains:

- `management_ip` (String) Management IP from returned GraphML.
- `sliver_id` (String) Sliver or reservation id when available.
- `state` (String) Sliver state from `/slivers`.
- `graph_node_id` (String) Graph node id used to correlate returned topology with slivers.
- `reservation_state` (String) Reservation state from returned GraphML when available.
- `error_message` (String) Reservation error message from returned GraphML when available.

## Permission Tags

The provider emits plan-time diagnostics for gated features when the required tag is absent from provider `project_tags`.

| Configuration | Required tag |
| --- | --- |
| `cores > 2` | `VM.NoLimitCPU` |
| `ram > 8` | `VM.NoLimitRAM` |
| `disk > 10` | `VM.NoLimitDisk` |
| Very large composite VM request | `VM.NoLimit` |
| `lifetime_hours > 24` | `Slice.NoLimitLifetime` |
| Nodes spanning more than one site | `Slice.Multisite` |
| GPU component | `Component.GPU` |
| FPGA component | `Component.FPGA` |
| NVME component | `Component.NVME` |
| Storage component | `Component.Storage` |
| SmartNIC `ConnectX-5` | `Component.SmartNIC_ConnectX_5` |
| SmartNIC `ConnectX-6` or `BlueField-2-ConnectX-6` | `Component.SmartNIC_ConnectX_6` |
| SmartNIC `ConnectX-7-100` | `Component.SmartNIC_ConnectX_7_100` |
| SmartNIC `ConnectX-7-400` | `Component.SmartNIC_ConnectX_7_400` |
| Network `bandwidth > 10000` | `Net.NoLimitBW` |
| `FABNetv4Ext` | `Net.FABNetv4Ext` |
| `FABNetv6Ext` | `Net.FABNetv6Ext` |
| `PortMirror` | `Net.PortMirroring` |

## Supported and Out-of-Scope Topologies

Implemented topology construction covers VM nodes, catalog components, component interface resolution, and top-level network services accepted by the schema.

The v0.1 plan intentionally leaves these out of resource CRUD:

- Facility ports.
- Switch nodes.
- Explicit physical links.
- Subinterface CRUD.

Some fixture-backed tests parse these shapes to keep compatibility with `fabric-go-fim`, but Terraform configuration for them is outside this provider version's resource schema.
