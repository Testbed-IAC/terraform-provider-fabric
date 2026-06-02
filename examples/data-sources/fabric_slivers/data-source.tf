# Read sliver runtime state for a provisioned FABRIC slice.
data "fabric_slivers" "science_gateway" {
  slice_id = "3f1b62c1-7a5e-4f46-b4ea-58d8df8c0d70"
}

output "science_gateway_management_ips" {
  value = [
    for sliver in data.fabric_slivers.science_gateway.slivers : sliver.management_ip
    if sliver.management_ip != ""
  ]
}
