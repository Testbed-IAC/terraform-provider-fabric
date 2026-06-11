terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = "~> 0.1"
    }
  }
}

variable "fabric_token" {
  description = "FABRIC ID token (JWT). Set via TF_VAR_fabric_token or a .tfvars file. Do not commit."
  type        = string
  sensitive   = true
}

provider "fabric" {
  token = var.fabric_token
}

# Omit token to resolve credentials from FABRIC_TOKEN_LOCATION, a token file at
# ~/.fabric/token.json or ~/work/fabric_config/id_token.json, or FABRIC_TOKEN. A
# token file refreshes automatically using its refresh_token; a static token does
# not. orchestrator_url and credmgr_url default to the production deployment.
