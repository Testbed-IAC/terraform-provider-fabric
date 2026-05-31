# Illustrative: typed facility-port discovery is implemented, but facility-port
# attachment blocks are not yet part of the slice schema in this checkout.
data "fabric_facility_ports" "renc" {
  includes = "RENC"
}

output "facility_ports" {
  value = data.fabric_facility_ports.renc.facility_ports
}
