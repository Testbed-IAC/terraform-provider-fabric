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
  ]
}

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

output "management_ips" {
  value = {
    renc = fabric_slice.multisite_l2sts.nodes["vm-renc"].management_ip
    uky  = fabric_slice.multisite_l2sts.nodes["vm-uky"].management_ip
  }
}
