terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}

variable "fabric_ssh_key" {
  type      = string
  sensitive = true
}

provider "fabric" {
  project_tags = [
    "Slice.Multisite",
    "Component.SmartNIC_ConnectX_6",
  ]
}

resource "fabric_slice" "smartnic_l2ptp" {
  name    = "tf-smartnic-l2ptp"
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
