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

# Fetch the raw advertised resource model. level 2 returns host and component
# detail; force_refresh bypasses the provider's cache. start_date/end_date scope
# availability to a planned reservation window.
data "fabric_resources" "window" {
  level         = 2
  force_refresh = true
  includes      = var.site
}

output "resource_model" {
  value = data.fabric_resources.window.model
}
