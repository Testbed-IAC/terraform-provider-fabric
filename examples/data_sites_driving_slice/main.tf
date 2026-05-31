terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}

provider "fabric" {}

data "fabric_sites" "available" {
  includes = "RENC,UKY"
}

resource "fabric_slice" "from_site_data" {
  name     = "tf-sites-data"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "vm1"
    site = data.fabric_sites.available.sites[0].name
  }
}

variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}
