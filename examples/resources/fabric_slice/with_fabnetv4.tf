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

# FABNetv4 is the FABRIC IPv4 routed network. It takes a gateway block and a
# subnet. FABNetv6 availability is site-dependent; check fabric_sites first.
resource "fabric_slice" "fabnetv4" {
  name     = "fabnetv4-example"
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
    name   = "fabnet"
    type   = "FABNetv4"
    site   = var.site
    subnet = "10.10.0.0/24"

    gateway {
      ipv4        = "10.10.0.1"
      ipv4_subnet = "10.10.0.0/24"
    }

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
