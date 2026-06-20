terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = ">= 0.1.1"
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

resource "fabric_slice" "nvme" {
  name     = "nvme-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "nvme-node"
    site      = var.site
    image_ref = "default_rocky_9"

    component {
      name = "nvme1"
      # P4510 is a 1TB NVMe drive added to the node as a passthrough device.
      type  = "NVME" # requires the Component.NVME capability tag in your FABRIC token
      model = "P4510"
    }
  }
}
