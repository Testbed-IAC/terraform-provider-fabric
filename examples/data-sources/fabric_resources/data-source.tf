# Fetch raw advertised resource data for two sites during a planned reservation
# window.
data "fabric_resources" "planned_window" {
  level         = 2
  force_refresh = true
  start_date    = "2026-06-15T14:00:00Z"
  end_date      = "2026-06-17T14:00:00Z"
  includes      = "RENC,UKY"
}

output "planned_resource_model" {
  value = data.fabric_resources.planned_window.model
}
