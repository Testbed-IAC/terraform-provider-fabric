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

variable "slice_id" {
  description = "FABRIC slice UUID whose slivers should be read."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Read per-sliver runtime state for a provisioned slice.
data "fabric_slivers" "all" {
  slice_id = var.slice_id
}

output "management_ips" {
  value = [
    for sliver in data.fabric_slivers.all.slivers : sliver.management_ip
    if sliver.management_ip != ""
  ]
}
