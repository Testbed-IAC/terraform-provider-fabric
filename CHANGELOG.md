# Changelog

All notable changes to this provider are documented in this file.

## [Unreleased]

### Fixed
- A `node` configured with `instance_type` no longer also emits the default
  2/8/10 capacities, which previously could cause the orchestrator to allocate
  the tiny default instead of the requested flavor.

### Documentation
- Removed the nonexistent `project_id`/`project_tags` provider attributes and
  the `FABRIC_PROJECT_ID`/`FABRIC_ORCHESTRATOR_URL` environment variables from
  the README; documented the actual authentication precedence, provider
  attributes, slice attributes, and import limitations.

## [0.1.0] - TBD

### Added
- `fabric_slice` resource for FABRIC slice lifecycle management.
- `fabric_slice` and `fabric_resources` data sources.
