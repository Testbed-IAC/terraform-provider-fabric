# Read FABRIC metrics while omitting projects that are not part of the reporting
# view for this workspace.
data "fabric_metrics" "overview" {
  excluded_projects = ["archive-training", "retired-demo"]
}

output "fabric_metrics_json" {
  value = data.fabric_metrics.overview.results
}
