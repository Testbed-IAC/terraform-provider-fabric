---
page_title: "Topology Model"
subcategory: "Guides"
description: |-
  How Terraform configuration maps to FABRIC GraphML.
---

# Topology Model

The provider translates Terraform configuration into FABRIC GraphML using `fabric-go-fim/pkg/topology`. Users do not write GraphML directly.

## Graph Identity

The topology builder creates a deterministic graph id seed:

```go
topology.NewWithID(topology.DeriveGraphID(sliceName))
```

This keeps generated topology ids stable for a given slice name. Renaming a slice changes the graph identity, so `name` forces replacement.

## Node Identity

Node identity is derived from the graph id and `node.name`. Changing `node.name` forces replacement.

FABRIC slivers are site-bound, so changing `node.site` also forces replacement.

## Defaults

When values are omitted, the builder uses these VM defaults:

| Field | Default |
| --- | --- |
| `cores` | `2` |
| `ram` | `8` |
| `disk` | `10` |
| `image_ref` | `default_rocky_9` |
| `image_type` | `qcow2` |

These defaults match the fixture-backed unit tests in `fabric-go-fim/testdata/fixtures`.

## Components

Components are created through the embedded component catalog. A component can be specified by explicit `type` and `model`, or by `fablib_name`.

`fablib_name` takes precedence. For example:

```terraform
component {
  name        = "snic"
  fablib_name = "NIC_ConnectX_6"
}
```

The provider resolves the FABlib name to a component type and catalog model before building GraphML.

## Network Interfaces

A `network.interface` block references an interface from a node.

The most explicit form is component plus port:

```terraform
interface {
  node      = "vm1"
  component = "nic1"
  port      = 0
}
```

The provider resolves this to `component.Interfaces()[port]`.

When `component` is omitted and `name` is set, the provider searches all interfaces reachable from the node by interface name.

When both `component` and `name` are omitted, the provider selects `node.InterfaceList()[port]`.

## Network Services

Top-level network blocks become FABRIC network service nodes. The schema accepts:

- `L2Bridge`
- `L2STS`
- `L2PTP`
- `FABNetv4`
- `FABNetv6`
- `FABNetv4Ext`
- `FABNetv6Ext`
- `PortMirror`

`L2Bridge` is the primary live acceptance-tested network service. Other types are represented in the schema and builder according to the v0.1 implementation plan.

## Drift Comparison

The provider compares desired and actual topology with `topology.DiffTopologies`. The comparison is semantic rather than raw XML text. This matters because returned GraphML can contain orchestrator-assigned ids while preserving the same named topology.
