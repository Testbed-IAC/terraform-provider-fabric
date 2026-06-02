variable "fabric_ssh_keys" {
  description = "SSH public keys to install on FABRIC slice nodes."
  type        = list(string)
  sensitive   = true
}

# -----------------------------------------------------------------------------
# Example 1: Minimal - one VM at one FABRIC site
# -----------------------------------------------------------------------------
# Request a small slice with one compute node at RENC and install the supplied
# SSH public keys for access after provisioning completes.
resource "fabric_slice" "single_vm" {
  name     = "ren-dev-single-vm"
  ssh_keys = var.fabric_ssh_keys

  node {
    name = "login"
    site = "RENC"
  }
}

# -----------------------------------------------------------------------------
# Example 2: Complete - compute, component, storage, routed network, facility
# -----------------------------------------------------------------------------
# Request a richer topology with explicit capacity, a SmartNIC component,
# storage, a routed FABNet service, and a facility port stitch.
resource "fabric_slice" "science_gateway" {
  name             = "ren-science-gateway"
  ssh_keys         = var.fabric_ssh_keys
  ssh_key_version  = 1
  lifetime_hours   = 48
  lease_start_time = "2026-06-15T14:00:00Z"

  node {
    name          = "gateway"
    site          = "RENC"
    host          = "renc-w1"
    instance_type = "fabric.c4.m8.d100"
    image_ref     = "default_ubuntu_22"
    image_type    = "qcow2"
    cores         = 4
    ram           = 8
    disk          = 100
    boot_script   = "echo FABRIC gateway node ready"

    post_boot_execute = [
      "sudo apt-get update",
      "sudo apt-get install -y iperf3",
    ]

    post_update = [
      "sudo systemctl restart ssh",
    ]

    labels {
      ipv4_subnet     = "192.0.2.0/24"
      instance_parent = "renc-w1"
    }

    component {
      name        = "smartnic"
      type        = "SmartNIC"
      model       = "ConnectX-6"
      fablib_name = "gateway-smartnic"

      labels {
        numa = 0
      }
    }

    storage {
      name       = "analysis-data"
      model      = "FABRIC_NetApp_1TB"
      auto_mount = true
    }

    route {
      subnet   = "198.51.100.0/24"
      next_hop = "192.0.2.1"
    }

    post_boot_upload {
      local_path  = "./bootstrap/gateway.sh"
      remote_path = "/home/ubuntu/bootstrap-gateway.sh"
    }
  }

  facility_port {
    name      = "RENC-ESnet"
    site      = "RENC"
    vlan      = "1720"
    bandwidth = 100
    mtu       = 9000

    labels {
      vlan_range = "1700-1799"
      local_name = "esnet-ren"
    }

    interface {
      name = "esnet-uplink"
      vlan = "1720"
    }
  }

  switch {
    name   = "campus-switch"
    site   = "RENC"
    nports = 2

    port_labels {
      vlan_range = "1700-1799"
    }
  }

  network {
    name       = "fabnet"
    type       = "FABNetv4"
    bandwidth  = 10
    site       = "RENC"
    technology = "AL2S"
    subnet     = "192.0.2.0/24"

    gateway {
      ipv4        = "192.0.2.1"
      ipv4_subnet = "192.0.2.0/24"
      mac         = "02:00:00:00:00:01"
    }

    interface {
      node      = "gateway"
      component = "smartnic"
      port      = 0
      name      = "gateway-data"

      labels {
        vlan = "1720"
      }

      sub_interface {
        name      = "gateway-data.1720"
        vlan      = "1720"
        bandwidth = 10
      }
    }

    interface {
      facility = "RENC-ESnet"
      port     = 0
      name     = "facility-uplink"
    }
  }
}
