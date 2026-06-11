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

variable "ssh_public_key" {
  description = "SSH public key to add to the target sliver."
  type        = string
  sensitive   = true
}

provider "fabric" {
  token = var.fabric_token
}

# Add an SSH key to a running sliver. The triggers map records the intent so a
# deliberate change re-runs the action; the same keys also apply to removekey.
resource "fabric_poa" "add_key" {
  sliver_id = var.target_sliver_id
  operation = "addkey"

  keys = [
    {
      key     = var.ssh_public_key
      comment = "research-ops-2026"
    },
  ]

  triggers = {
    rotation_batch = "2026-06-research-ops"
  }
}
