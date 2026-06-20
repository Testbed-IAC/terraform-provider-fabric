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

# A SmartNIC provides dedicated ports. Here it backs a FABNetv4 interface whose
# traffic is tagged onto a VLAN sub-interface.
resource "fabric_slice" "smart_nic" {
  name     = "smart-nic-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "nic-node"
    site      = var.site
    image_ref = "default_rocky_9"

    component {
      name = "nic1"
      # SmartNIC models: ConnectX-5, ConnectX-6, BlueField-2-ConnectX-6,
      # ConnectX-7-100, ConnectX-7-400. The tag is per-model: ConnectX-6
      # requires Component.SmartNIC_ConnectX_6.
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  network {
    name   = "fabnet"
    type   = "FABNetv4"
    site   = var.site
    subnet = "10.0.0.0/24"

    gateway {
      ipv4        = "10.0.0.1"
      ipv4_subnet = "10.0.0.0/24"
    }

    interface {
      node      = "nic-node"
      component = "nic1"
      port      = 0

      sub_interface {
        name = "nic-node-vlan100"
        vlan = "100"
      }
    }
  }
}
