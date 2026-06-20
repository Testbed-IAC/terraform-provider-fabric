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

provider "fabric" {
  token = var.fabric_token
}

# Decode advertised resources into typed site capacity, host, and component data.
# Omit includes/excludes to return every site your allocation can see.
data "fabric_sites" "all" {}

# Site names available to your allocation, for use as var.site elsewhere.
output "site_names" {
  value = [for site in data.fabric_sites.all.sites : site.name]
}

# Available GPU counts per site, keyed by component name (e.g. GPU/RTX6000).
output "gpu_availability" {
  value = {
    for site in data.fabric_sites.all.sites : site.name => {
      for component in site.components : component.name => component.available
      if startswith(component.name, "GPU/")
    }
  }
}
