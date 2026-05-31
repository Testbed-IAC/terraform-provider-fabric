terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}

provider "fabric" {}

resource "fabric_slice" "multi_key_vm" {
  name     = "tf-multi-key-vm"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "vm1"
    site = "RENC"
  }
}

variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}
