# Decode advertised facility ports at RENC and select the ESnet stitch port.
data "fabric_facility_ports" "renc_esnet" {
  name     = "RENC-ESnet"
  includes = "RENC"
}

output "renc_esnet_vlan_range" {
  value = one(data.fabric_facility_ports.renc_esnet.facility_ports).vlan_range
}
