terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}

provider "fabric" {}

resource "fabric_slice" "labeled_vm" {
  name     = "tf-labeled-vm"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "vm1"
    site = "RENC"

    labels {
      instance_parent = "renc-w1.fabric-testbed.net"
    }
  }
}

variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}
