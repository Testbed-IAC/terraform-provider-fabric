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

variable "slice_name" {
  description = "Slice name to look up. Resolves the first matching non-deleted slice."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Look up a slice by name. When several slices share a name, the first
# non-deleted match is returned.
data "fabric_slice" "by_name" {
  name = var.slice_name
}

output "slice_id" {
  value = data.fabric_slice.by_name.slice_id
}
