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

# L2Bridge is same-site only and OVS-backed. Each connected node needs a
# SharedNIC. Omitting type infers L2Bridge when all nodes share a site.
resource "fabric_slice" "l2bridge" {
  name     = "l2bridge-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "vm1"
    site      = var.site
    image_ref = "default_rocky_9"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  node {
    name      = "vm2"
    site      = var.site
    image_ref = "default_rocky_9"

    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name = "bridge"
    type = "L2Bridge"

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
