# Illustrative: FABNet gateway/L3VPN route controls are not exposed by the
# provider schema in this checkout.
resource "fabric_slice" "l3vpn" {
  name     = "tf-l3vpn"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "vm1"
    site = "RENC"
  }

  network {
    name = "fabnet"
    type = "FABNetv4"

    interface {
      node = "vm1"
    }
  }
}

variable "fabric_ssh_keys" {
  type      = list(string)
  sensitive = true
}
