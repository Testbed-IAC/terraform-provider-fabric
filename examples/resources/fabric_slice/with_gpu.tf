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

resource "fabric_slice" "gpu" {
  name     = "gpu-example"
  ssh_keys = [var.ssh_public_key]

  node {
    name      = "gpu-node"
    site      = var.site
    image_ref = "default_ubuntu_22"

    component {
      name = "gpu1"
      # Valid GPU models: RTX6000, Tesla T4, A40, A30. Model availability varies
      # by site and allocation; check fabric_sites before applying.
      type  = "GPU" # requires the Component.GPU capability tag in your FABRIC token
      model = "RTX6000"
    }
  }
}
