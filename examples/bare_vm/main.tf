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

resource "fabric_slice" "bare_vm" {
  name    = "tf-bare-vm"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }
}
