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

# Read the FABRIC metrics overview as a JSON string. excluded_projects drops
# named projects from the result.
data "fabric_metrics" "overview" {
  excluded_projects = []
}

output "metrics_json" {
  value = data.fabric_metrics.overview.results
}
