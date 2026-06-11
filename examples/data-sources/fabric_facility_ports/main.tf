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

variable "site" {
  description = "FABRIC site name from your project allocation. Discover available sites with the fabric_sites data source."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Decode advertised facility ports at a site. Use the returned name and
# vlan_range when wiring a facility_port block in a fabric_slice.
data "fabric_facility_ports" "site" {
  includes = var.site
}

output "facility_ports" {
  value = {
    for port in data.fabric_facility_ports.site.facility_ports : port.name => port.vlan_range
  }
}
