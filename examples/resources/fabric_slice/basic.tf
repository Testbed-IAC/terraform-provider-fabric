terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = "~> 0.1"
    }
  }
}

variable "fabric_token" {
  description = "FABRIC ID token (JWT). Set via TF_VAR_fabric_token. Do not commit."
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key to install on slice nodes."
  type        = string
}

variable "site" {
  description = "FABRIC site name from your project allocation. Discover available sites with the fabric_sites data source."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Discover sites your allocation can use rather than hardcoding one.
data "fabric_sites" "available" {}

resource "fabric_slice" "quickstart" {
  name     = "quickstart-single-vm"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "vm1"
    site      = var.site
    image_ref = "default_rocky_9"
  }
}

output "management_ip" {
  value = fabric_slice.quickstart.nodes["vm1"].management_ip
}
