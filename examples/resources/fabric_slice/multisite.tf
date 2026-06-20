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

variable "site_a" {
  description = "First FABRIC site from your project allocation."
  type        = string
}

variable "site_b" {
  description = "Second FABRIC site from your project allocation, different from site_a."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Two VMs at different sites with no network service between them. Placing nodes
# at more than one site requires the Slice.Multisite capability tag.
resource "fabric_slice" "multisite" {
  name     = "multisite-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "vm1"
    site      = var.site_a
    image_ref = "default_rocky_9"
  }

  node {
    name      = "vm2"
    site      = var.site_b
    image_ref = "default_rocky_9"
  }
}
