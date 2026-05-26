---
page_title: "fabric_resources Data Source"
subcategory: ""
description: |-
  Returns the FABRIC available resources model.
---

# fabric_resources

Use this data source to fetch the FABRIC available resources model from the orchestrator. The provider passes `level` and `force_refresh` directly to `GET /resources`.

The returned `model` is the raw model string from the orchestrator response.

## Example Usage

```terraform
data "fabric_resources" "current" {
  level         = 1
  force_refresh = false
}

output "resources_model" {
  value = data.fabric_resources.current.model
}
```

## Schema

### Optional

- `level` (Number) Resource detail level. Defaults to `1` in the data source read path when omitted.
- `force_refresh` (Boolean) Whether to ask the orchestrator for current resource information instead of cached data. Defaults to `false` when omitted.

### Read-Only

- `id` (String) Static id value, `resources`.
- `model` (String) Raw resource model returned by the orchestrator.
