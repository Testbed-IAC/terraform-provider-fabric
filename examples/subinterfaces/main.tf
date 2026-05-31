# Illustrative: sub-interface blocks are not exposed by the provider schema in
# this checkout.
resource "fabric_slice" "subinterfaces" {
  name     = "tf-subinterfaces"
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
