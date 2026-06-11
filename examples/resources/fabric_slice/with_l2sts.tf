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

# L2STS is a cross-site site-to-site L2 service. It requires dedicated ports
# (SmartNIC or facility) on each node and the Slice.Multisite capability tag.
resource "fabric_slice" "l2sts" {
  name     = "l2sts-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "vm1"
    site      = var.site_a
    image_ref = "default_rocky_9"

    component {
      name  = "nic1"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name      = "vm2"
    site      = var.site_b
    image_ref = "default_rocky_9"

    component {
      name  = "nic1"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  # Nodes span two sites, so the slice requires the Slice.Multisite tag.
  network {
    name = "sts"
    type = "L2STS"

    interface {
      node      = "vm1"
      component = "nic1"
    }

    interface {
      node      = "vm2"
      component = "nic1"
    }
  }
}
