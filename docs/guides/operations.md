---
page_title: "Operational Behavior"
subcategory: "Guides"
description: |-
  Polling, update, drift, import, and delete behavior.
---

# Operational Behavior

This guide summarizes behavior that is important when running the provider in automation.

## Polling

The provider polls slice state after create, update, and delete. The poller is context-aware and returns when the operation timeout expires or the Terraform context is canceled.

Default operation timeout when not configured:

| Operation | Default |
| --- | --- |
| Create | 30 minutes |
| Read | Framework default unless configured |
| Update | 30 minutes |
| Delete | 30 minutes |

Example:

```terraform
resource "fabric_slice" "example" {
  name    = "tf-example"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }

  timeouts {
    create = "45m"
    update = "45m"
    delete = "30m"
  }
}
```

## State Handling

| Orchestrator state | Provider behavior |
| --- | --- |
| `Nascent` | Poll or preserve during read. |
| `Configuring` | Poll or preserve during read. |
| `AllocatedOK` | Intermediate state. |
| `AllocatedError` | Terminal failure during create. |
| `StableOK` | Healthy terminal state. |
| `StableError` | Warning during read, state preserved. |
| `Modifying` | Poll during update. |
| `ModifyOK` | Accept modify. Read attempts recovery. |
| `ModifyError` | Accept modify anyway, warning emitted. |
| `Closing` | Removed from Terraform state during read. |
| `Dead` | Removed from Terraform state during read. |
| `ClosingError` | Defensive delete-terminal state in provider code. |

`ClosingError` is not part of the current OpenAPI enum, but the implementation treats it as a defensive terminal delete state as described in the implementation plan.

## Update and Accept

FABRIC modify is a two-step workflow:

1. Submit the desired topology through modify.
2. Accept the modify result.

The provider always accepts after `ModifyOK`. It also accepts after `ModifyError`, because the orchestrator accept operation prunes failed resources.

## Drift

Drift warnings do not fail read. This is deliberate. Out-of-band portal changes, sliver failures, and expiration can all create drift. The warning is intended to make the difference visible while still allowing Terraform to produce a plan.

## Import

Import sets `id` and `slice_id`. The next refresh reads slice fields from the orchestrator.

```shell
terraform import fabric_slice.example <slice_id>
```

After import, the HCL configuration should describe the imported topology. Otherwise Terraform will plan changes to reconcile the imported slice to the configuration.
