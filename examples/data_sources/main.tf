terraform {
  required_providers {
    fabric = {
      source = "Testbed-IAC/fabric"
    }
  }
}

variable "slice_id" {
  type = string
}

data "fabric_slice" "existing" {
  slice_id = var.slice_id
}

data "fabric_resources" "current" {
  level         = 1
  force_refresh = false
}

output "slice_state" {
  value = data.fabric_slice.existing.state
}

output "resources_model" {
  value = data.fabric_resources.current.model
}
