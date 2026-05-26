terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}

variable "fabric_token" {
  type      = string
  sensitive = true
  default   = null
}

variable "fabric_project_id" {
  type    = string
  default = null
}

variable "fabric_ssh_key" {
  type      = string
  sensitive = true
}

provider "fabric" {
  token      = var.fabric_token
  project_id = var.fabric_project_id

  project_tags = [
    "VM.NoLimitCPU",
    "VM.NoLimitRAM",
    "VM.NoLimitDisk",
  ]
}

resource "fabric_slice" "advanced_vm" {
  name           = "tf-advanced-vm"
  ssh_key        = var.fabric_ssh_key
  lifetime_hours = 24

  node {
    name       = "vm1"
    site       = "RENC"
    image_ref  = "default_rocky_9"
    image_type = "qcow2"
    cores      = 4
    ram        = 16
    disk       = 50
  }

  timeouts {
    create = "45m"
    update = "45m"
    delete = "30m"
  }
}

output "slice_id" {
  value = fabric_slice.advanced_vm.slice_id
}

output "vm1" {
  value = fabric_slice.advanced_vm.nodes["vm1"]
}
