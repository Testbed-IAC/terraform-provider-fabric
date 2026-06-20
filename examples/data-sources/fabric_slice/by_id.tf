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
  description = "FABRIC slice UUID from the portal or orchestrator slice list."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Look up a slice by UUID. slice_id takes precedence over id, and id over name.
data "fabric_slice" "by_id" {
  slice_id = var.slice_id
}

output "slice_state" {
  value = data.fabric_slice.by_id.state
}
