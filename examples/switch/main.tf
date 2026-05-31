# Illustrative: switch node CRUD is not exposed by the provider schema yet.
# Use this as the intended shape once switch support lands.
resource "fabric_slice" "switch_example" {
  name     = "tf-switch"
  ssh_keys = var.fabric_ssh_keys

  # switch {
  #   name = "sw1"
  #   site = "RENC"
  # }
}

variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}
