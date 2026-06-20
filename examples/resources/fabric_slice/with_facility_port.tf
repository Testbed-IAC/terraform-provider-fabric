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

variable "facility_port_name" {
  description = "Advertised facility port name. Discover ports with the fabric_facility_ports data source."
  type        = string
}

variable "facility_vlan" {
  description = "VLAN tag within the facility port's advertised vlan_range."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# A facility port stitches the slice to an external network. The node connects
# over a SmartNIC dedicated port; the facility port is named from the advertised
# fabric_facility_ports data and tagged onto a VLAN in its advertised range.
resource "fabric_slice" "facility" {
  name     = "facility-port-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "vm1"
    site      = var.site
    image_ref = "default_rocky_9"

    component {
      name  = "nic1"
      type  = "SmartNIC"
      model = "ConnectX-6"
    }
  }

  facility_port {
    name      = var.facility_port_name
    site      = var.site
    vlan      = var.facility_vlan
    bandwidth = 10

    interface {
      name = "facility-uplink"
      vlan = var.facility_vlan
    }
  }

  network {
    name = "stitch"
    type = "L2STS"

    interface {
      node      = "vm1"
      component = "nic1"
    }

    interface {
      facility = var.facility_port_name
    }
  }
}
