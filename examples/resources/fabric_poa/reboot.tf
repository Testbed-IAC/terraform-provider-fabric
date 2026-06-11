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

variable "target_sliver_id" {
  description = "FABRIC sliver UUID to run the operational action against."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Reboot a provisioned node sliver. Replacing this resource (changing sliver_id,
# operation, or triggers) re-runs the reboot; deleting it only forgets state.
resource "fabric_poa" "reboot" {
  sliver_id = var.target_sliver_id
  operation = "reboot"
}
