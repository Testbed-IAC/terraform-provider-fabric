---
page_title: "fabric_slice Data Source"
subcategory: ""
description: |-
  Looks up a FABRIC slice by id or name.
---

# fabric_slice

Use this data source to read metadata for an existing FABRIC slice. The data source looks up by `slice_id` when set. If both `slice_id` and `name` are set, `slice_id` wins.

When only `name` is set, the provider lists active and error-state slices by exact name and returns the first match.

## Example Usage

### Lookup by Slice ID

```terraform
data "fabric_slice" "existing" {
  slice_id = "11111111-2222-3333-4444-555555555555"
}

output "slice_state" {
  value = data.fabric_slice.existing.state
}
```

### Lookup by Name

```terraform
data "fabric_slice" "by_name" {
  name = "tf-bare-vm"
}
```

## Schema

### Optional

- `id` (String) Slice id. This is accepted as an alias for `slice_id`.
- `slice_id` (String) FABRIC slice UUID. Preferred lookup key.
- `name` (String) Slice name used for exact-match lookup when no id is set.

At least one of `id`, `slice_id`, or `name` must be configured.

### Read-Only

- `state` (String) Orchestrator slice state.
- `graph_id` (String) Orchestrator graph id.
- `lease_start_time` (String) Lease start time returned by the orchestrator.
- `lease_end_time` (String) Lease end time returned by the orchestrator.

## Lookup States

Name lookup searches slices in these states:

- `Nascent`
- `Configuring`
- `StableError`
- `StableOK`
- `Modifying`
- `ModifyOK`
- `ModifyError`
- `AllocatedOK`
- `AllocatedError`

Dead slices are not selected by name lookup.
