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
