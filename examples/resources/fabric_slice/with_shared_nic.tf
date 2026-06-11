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

variable "ssh_public_key" {
  description = "SSH public key to install on slice nodes."
  type        = string
}

variable "site" {
  description = "FABRIC site name from your project allocation. Discover available sites with the fabric_sites data source."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

resource "fabric_slice" "shared_nic" {
  name     = "shared-nic-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "nic-node"
    site      = var.site
    image_ref = "default_rocky_9"

    # SharedNIC components require no capability tag. Connecting one to a
    # same-site L2Bridge auto-generates the backing OVS interface.
    component {
      name  = "nic1"
      type  = "SharedNIC"
      model = "ConnectX-6"
    }
  }
}
