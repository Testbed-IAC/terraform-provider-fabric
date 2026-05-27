terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}
variable "fabric_token" {
  description = "FABRIC bearer JWT from https://portal.fabric-testbed.net → Experiments → Tokens"
  type        = string
  sensitive   = true
  validation {
    condition     = length(var.fabric_token) > 0 && startswith(var.fabric_token, "eyJ")
    error_message = "fabric_token must be a valid FABRIC JWT (starts with 'eyJ'). Get one from the FABRIC portal."
  }
}

variable "fabric_project_id" {
  description = "FABRIC project UUID from https://portal.fabric-testbed.net → Projects"
  type        = string
  validation {
    condition     = can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.fabric_project_id))
    error_message = "fabric_project_id must be a valid UUID."
  }
}

variable "fabric_ssh_key" {
  description = "SSH public key to install on the FABRIC slice"
  type        = string
  sensitive   = true
}

provider "fabric" {
  token      = var.fabric_token
  project_id = var.fabric_project_id
}

resource "fabric_slice" "bare_vm" {
  name    = "tf-bare-vm"
  ssh_key = var.fabric_ssh_key

  node {
    name = "vm1"
    site = "UTAH"
  }
}
