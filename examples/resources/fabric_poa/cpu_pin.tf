terraform {
  required_providers {
    fabric = {
      source  = "Testbed-IAC/fabric"
      version = ">= 0.1.1"
    }
  }
}

variable "fabric_token" {
  description = "FABRIC ID token (JWT). Set via TF_VAR_fabric_token. Do not commit."
  type        = string
  sensitive   = true
}

variable "target_sliver_id" {
  description = "FABRIC sliver UUID to run the operational action against."
  type        = string
}

provider "fabric" {
  token = var.fabric_token
}

# Pin guest vCPUs to host CPUs. vcpu_cpu_map applies to the cpupin and numatune
# operations.
resource "fabric_poa" "cpu_pin" {
  sliver_id = var.target_sliver_id
  operation = "cpupin"

  vcpu_cpu_map = [
    {
      vcpu = "0"
      cpu  = "2"
    },
    {
      vcpu = "1"
      cpu  = "3"
    },
  ]
}
