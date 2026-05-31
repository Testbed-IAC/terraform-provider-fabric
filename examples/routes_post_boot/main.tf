# Illustrative: post-boot scripts and route blocks are not exposed by the
# provider schema in this checkout.
resource "fabric_slice" "routes" {
  name     = "tf-routes"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "vm1"
    site = "RENC"
  }
}

variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}
