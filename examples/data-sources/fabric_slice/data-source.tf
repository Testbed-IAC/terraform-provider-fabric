# Look up a provisioned FABRIC slice by its UUID from the FABRIC portal or
# orchestrator slice list.
data "fabric_slice" "science_gateway" {
  slice_id = "3f1b62c1-7a5e-4f46-b4ea-58d8df8c0d70"
}

output "science_gateway_state" {
  value = data.fabric_slice.science_gateway.state
}
