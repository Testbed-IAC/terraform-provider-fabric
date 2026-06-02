# Decode advertised resources into typed site capacity data for the sites used
# by a planned experiment.
data "fabric_sites" "selected" {
  includes      = "RENC,UKY"
  force_refresh = true
}

output "selected_site_names" {
  value = [for site in data.fabric_sites.selected.sites : site.name]
}
