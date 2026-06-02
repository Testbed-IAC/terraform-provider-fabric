terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = "~> 0.1"
    }
  }
}

# Credentials should come from FABRIC_TOKEN_LOCATION, ~/.fabric/token.json,
# ~/work/fabric_config/id_token.json, or FABRIC_TOKEN in normal usage.
provider "fabric" {
  orchestrator_url = "https://orchestrator.fabric-testbed.net"
  credmgr_url      = "https://cm.fabric-testbed.net"
}
