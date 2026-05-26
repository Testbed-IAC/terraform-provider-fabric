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

resource "fabric_slice" "example" {
  name    = "tf-example"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "RENC"
  }
}
