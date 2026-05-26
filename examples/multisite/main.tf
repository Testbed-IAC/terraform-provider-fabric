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

resource "fabric_slice" "multisite" {
  name    = "tf-multisite"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm-renc"
    site = "RENC"
  }

  node {
    name = "vm-uky"
    site = "UKY"
  }
}
