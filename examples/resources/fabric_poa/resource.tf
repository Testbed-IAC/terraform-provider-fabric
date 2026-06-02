variable "target_sliver_id" {
  description = "FABRIC sliver UUID that should receive the operational action."
  type        = string
}

variable "replacement_ssh_key" {
  description = "SSH public key to add to or remove from the target sliver."
  type        = string
  sensitive   = true
}

# -----------------------------------------------------------------------------
# Example 1: Minimal - reboot one sliver
# -----------------------------------------------------------------------------
# Reboot a provisioned node sliver by UUID. Replacing this resource re-runs the
# reboot request, so keep triggers stable unless a new action is intended.
resource "fabric_poa" "reboot_node" {
  sliver_id = var.target_sliver_id
  operation = "reboot"
}

# -----------------------------------------------------------------------------
# Example 2: Complete - add a key and include replacement triggers
# -----------------------------------------------------------------------------
# Add an SSH public key to a target sliver. The trigger records the intended
# rotation batch so changing it deliberately re-runs the action.
resource "fabric_poa" "add_access_key" {
  sliver_id = var.target_sliver_id
  operation = "addkey"

  node_set = ["gateway"]
  bdf      = ["0000:41:00.0"]

  keys = [{
    key     = var.replacement_ssh_key
    comment = "research-ops-2026"
  }]

  vcpu_cpu_map = [{
    vcpu = "0"
    cpu  = "2"
  }]

  triggers = {
    rotation_batch = "2026-06-research-ops"
  }
}
