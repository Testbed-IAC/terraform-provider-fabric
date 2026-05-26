---
page_title: "FABRIC Permission Tags"
subcategory: "Guides"
description: |-
  Project tag validation used by the provider.
---

# FABRIC Permission Tags

FABRIC project tags grant access to larger capacity requests and specialized resource classes. The provider validates known tag requirements locally before submitting GraphML.

Declare available tags in the provider:

```terraform
provider "fabric" {
  token      = var.fabric_token
  project_id = var.fabric_project_id

  project_tags = [
    "VM.NoLimitDisk",
    "Slice.Multisite",
  ]
}
```

If a required tag is missing, Terraform returns an attribute diagnostic naming the exact tag. The provider does not silently remove or downgrade the requested resource.

## Tag Matrix

| Request | Required tag |
| --- | --- |
| VM cores above default | `VM.NoLimitCPU` |
| VM RAM above default | `VM.NoLimitRAM` |
| VM disk above default | `VM.NoLimitDisk` |
| Very large composite VM request | `VM.NoLimit` |
| Slice lifetime above 24 hours | `Slice.NoLimitLifetime` |
| Nodes at multiple sites | `Slice.Multisite` |
| GPU | `Component.GPU` |
| FPGA | `Component.FPGA` |
| NVME | `Component.NVME` |
| Storage | `Component.Storage` |
| SmartNIC ConnectX-5 | `Component.SmartNIC_ConnectX_5` |
| SmartNIC ConnectX-6 | `Component.SmartNIC_ConnectX_6` |
| SmartNIC ConnectX-7 100G | `Component.SmartNIC_ConnectX_7_100` |
| SmartNIC ConnectX-7 400G | `Component.SmartNIC_ConnectX_7_400` |
| Network bandwidth above 10000 | `Net.NoLimitBW` |
| FABNetv4 external network | `Net.FABNetv4Ext` |
| FABNetv6 external network | `Net.FABNetv6Ext` |
| Port mirror service | `Net.PortMirroring` |

## Notes

The provider currently treats `BlueField-2-ConnectX-6` as requiring `Component.SmartNIC_ConnectX_6`.

Permission validation is a preflight check. The orchestrator remains the final authority and can still reject a request for policy or availability reasons.
