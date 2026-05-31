# FABRIC Terraform Provider — Full-Port Implementation Plan

> **Audience:** A Go engineer with no prior context on this repository.
> **Goal:** Take the current "thin subset" provider to a fully-featured, professionally
> engineered Terraform provider for the FABRIC testbed, with parity against FABlib/FIM
> where it makes sense for declarative infrastructure-as-code.
>
> This document is self-contained. Every gap is validated against source with
> `file:line` references. Every workstream lists the exact files to touch, the target
> Terraform schema, the FIM mapping, the tests required, and acceptance criteria.

---

## 0. Repository Map and Build Context

Three sibling Go modules under `/Users/tylergeiger/workspace/fabric-tools/`, wired with
local `replace` directives (monorepo-style; no published versions):

| Module | Path | Module path | Role |
|--------|------|-------------|------|
| Provider | `terraform-provider-fabric/` | `github.com/Testbed-IAC/terraform-provider-fabric` | Terraform Plugin Framework provider |
| FIM | `fabric-go-fim/` | `github.com/Testbed-IAC/fabric-go-fim` | Go port of the FABRIC Information Model (topology → GraphML) |
| Orchestrator client | `fabric-orchestrator-go-client/` | `github.com/Testbed-IAC/fabric-orchestrator-go-client` | OpenAPI-generated REST client |

Two Python repos are **reference-only** (the source of truth for "what features exist"):
- `informationmodel/` — canonical FIM (enums, Labels, Capacities, Gateway, service constraints).
- `fabrictestbed-extensions/` — FABlib (the user-facing Python SDK whose surface we are porting).

Provider `go.mod`: Go 1.24.0; framework `terraform-plugin-framework v1.18.0`,
`-validators v0.15.0`, `-timeouts v0.5.0`, docs via `terraform-plugin-docs v0.24.0`.
Replace directives point FIM and the client at `../fabric-go-fim` and
`../fabric-orchestrator-go-client`.

Build/dev entry points (already present):
- `GNUmakefile`: `build`, `install`, `test`, `testacc` (`TF_ACC=1 … -run TestAccFabric`), `lint`, `fmt`, `docs`, `tidy`.
- `.golangci.yml`: enables `errcheck, staticcheck, unused, govet`. **Expand this** (see §11).
- `generate.go`: `//go:generate tfplugindocs generate --provider-name fabric`.

### Current provider surface (baseline — what exists today)

- **1 resource:** `fabric_slice` (`internal/provider/resource_slice.go`, schema in
  `resource_slice_schema.go`). Supports: top-level slice fields; `node` blocks
  (name/site/instance_type/image/cores/ram/disk) with `component` sub-blocks
  (name/type/model/fablib_name); `network` blocks (name/type/bandwidth/mirror_from/
  mirror_direction) with `interface` sub-blocks (node/component/port/name).
- **2 data sources:** `fabric_slice` (lookup) and `fabric_resources` (raw catalog
  `model` string + `level`/`force_refresh`).
- Conversion: `topology_builder.go` (`buildTopology`) → FIM `Topology` → GraphML string.
- Client wrapper: `internal/fabricclient/adapter.go` (already supports `[]ssh_keys` via
  `SlicesCreatesPost`).
- Supporting packages: `poller/`, `permission/`, `testutil/`.

Two prior planning docs exist at repo root: `plan.md` (original build plan, phases 1–5)
and `PLAN2.md` (refactor reconciliation notes). **This document supersedes both for the
full-port effort** and should be treated as the active plan. Do not delete the old docs;
add a one-line pointer at the top of each to this file (§13 deliverables).

---

## 1. Validation of the Reported Gaps (re-scan results)

Each item below was re-verified against source. Status legend:
**CONFIRMED** (real gap), **CONFIRMED+NUANCE** (real but the framing changes the work),
**PARTLY WRONG** (claim inaccurate), **ALREADY DONE** (already implemented).

### Highest priority

1. **Typed site/resource data sources — CONFIRMED.**
   `fabric_resources` exposes only `level`, `force_refresh`, and an opaque `model` string
   (`data_source_resources.go`; adapter `GetResources` ignores everything else,
   `adapter.go:289-318`). The orchestrator `ResourcesGet` supports `level`, `force_refresh`,
   `start_date`, `end_date`, `includes`, `excludes`, and there is a separate
   `PortalresourcesGet` with `graph_format` (`api_resources.go`). FIM has a *portal*
   `ResourcesSummary` decoder (`pkg/catalog/resources_summary.go:1-204`) but **no parser
   for the orchestrator's advertised-topology GraphML**. Typed data sources need a new
   GraphML advertised-topology decoder in FIM. → Workstream **D1, D2**.

2. **Facility ports — CONFIRMED.** FIM is ready: `FacilityOpts`
   (`pkg/topology/options.go:108-117`) and `Topology.AddFacility`
   (`pkg/topology/topology.go:117`). FABlib `add_facility_port` takes `vlan`, `labels`,
   `peer_labels`, `bandwidth`, `mtu` (`fablib/slice.py:1250-1289`). Provider exposes
   nothing. → Workstream **B3**.

3. **Labels not exposed — CONFIRMED.** FIM models the full label set
   (`pkg/sliver/value_types.go:66-89`: vlan, inner_vlan, vlan_range, ipv4/6
   addr/range/subnet, mac, asn, bgp_key, account_id, region, local_name, local_type,
   device_name, numa, bdf, usb_id). Provider has no `labels` block anywhere. → Workstream **A1**.

4. **L3 network metadata incomplete — CONFIRMED.** Provider `network` has only
   type/bandwidth/mirror_* (`resource_slice_schema.go`). FIM `NetworkServiceOpts` already
   supports `Gateway`, `Technology`, `Site`, `ERO`, `PathInfo`, `Labels`, `Capacities`
   (`pkg/topology/options.go:71-86`); `ServiceType` enum already includes `L3VPN`,
   `L2Path`, `L2Multisite`, `VLAN`, `MPLS` (`pkg/sliver/enums.go:25`). Provider's
   `network.type` `OneOf` is missing `L3VPN` (and the others). → Workstream **B1, B2**.

5. **L2 auto-selection missing — CONFIRMED + NUANCE.** This logic exists **only** in
   FABlib (`network_service.py:__calculate_l2_nstype`, lines 141-199) and **does not
   exist in Go at all** — FIM requires an explicit `ServiceType` and performs no
   inference. So this is *net-new FIM work* (a pure function in `fabric-go-fim`), then a
   thin provider wiring to make `network.type` optional. → Workstream **B2** (+ FIM helper).

### Likely wrong / fragile

6. **Lease time format — CONFIRMED (real risk).** Provider validates `lease_start_time`
   as RFC3339 (`resource_slice_schema.go`, regex ~line 60) and passes the string
   unchanged to `.LeaseStartTime(string)` / `.LeaseEndTime(string)`. Because these client
   methods take **strings**, the client's `time.Time → RFC3339Nano` coercion
   (`client.go:184`) never runs — whatever string we pass goes into the query verbatim.
   Swagger documents the format as `"2023-01-01 16:20:15 +00:00"`
   (`swagger.yaml` ~145/154/261/270), **not** RFC3339. → Workstream **C1** (FABRIC time
   type in FIM, normalize on the way out, parse on the way back).

7. **Port-mirror direction naming — PARTLY WRONG (low priority).** Provider accepts
   `Both`/`RX_Only`/`TX_Only`, which is the **correct FIM wire value**
   (`pkg/sliver/enums.go:135` MirrorBoth/MirrorRXOnly/MirrorTXOnly serialize to those
   strings). FABlib's `rx`/`tx`/`both` is just its own sugar. This is **not a bug**; it is
   an optional UX alias. → Workstream **B4** (optional, accept aliases + normalize).

8. **Host placement missing — CONFIRMED.** FABlib `set_host` → `Labels.instance_parent`
   (`fablib/node.py:858-873`). FIM `Labels.Instance`/`InstanceParent` exist
   (`value_types.go`). Provider has no `node.host`. Implement as sugar over the node
   labels block once A1 lands. → Workstream **A1 / B-host**.

### Broader coverage gaps

9. **Switch / P4 nodes — CONFIRMED (FIM ready).** `Topology.AddSwitch` +`SwitchOpts`
   (`topology.go:150`, `options.go:125-134`). FABlib `add_switch`
   (`slice.py:1488-1555`). → Workstream **B5**.

10. **Storage — CONFIRMED.** FABlib `add_storage`/`enable_storage`
    (`node.py:3884`, `3947`). FIM models `ComponentTypeStorage`/`NVME` via catalog. The
    CephFS *post-boot* helper state is FABlib runtime sugar — model the declarative part
    (a Storage component / `add_storage`) but treat CephFS mount orchestration as
    out-of-scope for a declarative provider (document it). → Workstream **A2**.

11. **Node userdata / post-boot / routes — CONFIRMED + NUANCE.** FIM stores `UserData`
    as an **opaque `[]byte`** and `BootScript` as an opaque string
    (`pkg/sliver/slivers.go`). FABlib persists routes and post-boot tasks **inside** the
    user-data JSON under a `fablib_data` envelope (`node.py:4003-4101`). To port this we
    must define that JSON schema as typed Go in FIM (so both writer and reader agree), not
    just pass a blob. → Workstream **A3** (FIM typed user-data) + **A4** (provider blocks).
    Note: a Terraform provider **declares** routes/post-boot tasks into user-data; it does
    **not** SSH in and execute them (that is FABlib runtime behavior). Document this
    boundary explicitly.

12. **Sub-interfaces / VLAN child interfaces — CONFIRMED (FIM ready).**
    `Interface.AddChildInterface` (`pkg/topology/facades.go:403`) requires parent
    `DedicatedPort` + child VLAN. FABlib `add_sub_interface`
    (`fablib/interface.py:1451-1499`). Provider cannot model child interfaces. → Workstream **A5**.

13. **Multiple SSH keys — CONFIRMED + NUANCE (shallow).** The **adapter already sends
    `[]ssh_keys`** (`adapter.go:55,60`, `SlicesPost{GraphModel, SshKeys}`). Only the
    Terraform schema is single-valued (`ssh_key`). Add `ssh_keys` list, keep `ssh_key`
    as a deprecated alias for one release. → Workstream **C2**.

14. **POA & metrics — CONFIRMED.** Client exposes `PoasAPI` (operations: cpuinfo,
    numainfo, cpupin, numatune, reboot, addkey, removekey, rescan; `api_poas.go`) and
    `MetricsAPI` (`MetricsOverviewGet`). Provider has none. Model POA as an **action-style
    resource** and metrics as a **data source** (§ Workstream **E**). These are explicitly
    lower priority and must not be forced into the slice schema.

### Additional gaps found during re-scan (not in the original list)

15. **`PortalresourcesGet` and resources query filters unused.** Even the existing
    `fabric_resources` data source could expose `start_date`/`end_date`/`includes`/
    `excludes`/`graph_format`. Fold into **D1**.

16. **`SliversGet` / per-sliver detail underused.** The adapter has `GetSlivers`
    (`adapter.go:265`) but the slice resource derives node outputs from the slice model
    only. A `fabric_slivers` data source (or richer `nodes` outputs) would surface
    `management_ip`, `sliver_id`, join/pending state per node. Fold into **D3**.

17. **Tags.** FIM `NodeOpts.Tags` exists (`options.go`). Not exposed. Low priority; add an
    optional `tags` list on node/network if desired (Workstream A — optional).

18. **`AddLink` / explicit L1/L2 paths.** FIM `AddLink` (`topology.go:225`,
    `LinkOpts`). Rarely used in declarative slices; document as out-of-scope for v1 unless
    a concrete user need appears. (Listed for completeness; **not scheduled**.)

19. **Boot script.** FIM `NodeOpts.BootScript` (≤1024 bytes). Distinct from post-boot
    tasks. Cheap to expose as `node.boot_script`. Fold into **A2**.

---

## 2. Architecture & Boundary Rules (read before coding)

The single most important rule: **reusable FABRIC/FIM behavior lives in
`fabric-go-fim`; the provider only does Terraform.** Any logic that FABlib performs in
Python and that is *topology semantics* must be ported into FIM with its own tests, then
called from the provider. The provider must not re-derive FABRIC rules inline.

`fabric-go-fim` owns:
- Topology semantics, GraphML (de)serialization, deterministic ordering.
- Labels, Capacities, Gateway, CapacityHints, Flags.
- **NEW:** L2/L3 service-type inference (port of `__calculate_l2_nstype` + L3 mapping).
- **NEW:** typed user-data envelope (`fablib_data`: routes, post-boot tasks, storage flags).
- **NEW:** advertised-topology (resource catalog) GraphML decoding into typed sites/hosts/
  components/facility-ports/links.
- **NEW:** a FABRIC time type (parse/format the `"2006-01-02 15:04:05 -07:00"` form and
  RFC3339, normalize to the orchestrator's expected wire format).
- Topology validation and constraint checks.

`terraform-provider-fabric` owns:
- Schema design (snake_case), models, validators, plan modifiers, diagnostics.
- CRUD/Import/refresh lifecycle, state shape, plan-time validation.
- Mapping Terraform models ↔ FIM option structs ↔ orchestrator requests.
- Examples, docs, Terraform UX.

If you are about to write FABRIC logic inside `internal/provider`, stop and put it in FIM.

---

## 3. Engineering Standards (enforced for every workstream)

These apply repo-wide and are acceptance gates for every PR. (Adapted to this codebase.)

### Core quality rules
- No happy-path-only code. No placeholder abstractions for imagined futures.
- No test that cannot fail for a real regression; never weaken a test to go green.
- No duplicated FABRIC/FIM logic in the provider — it belongs in `fabric-go-fim`.
- Resources must behave like declarative infrastructure, not shell scripts.
- No `fmt.Println`; diagnostics for users, `tflog` (structured) for maintainers.
- No dead code, stale examples, misleading comments, or TODOs hiding broken behavior
  (a TODO is allowed only with a clear, honest explanation of the limitation).

### Go
- Small, responsibility-based packages; short functions; concrete types over needless
  interfaces; consumer-side interfaces only when needed.
- `context.Context` on all provider/client/API calls.
- Wrap errors with `%w` and useful context (resource, op, site, slice/sliver id, field
  path). Error strings lowercase, no trailing punctuation. Never panic on
  config/API/normal failures. Never swallow or double-log-and-return.
- No global mutable state. Deterministic ordering for any generated output
  (GraphML, maps, computed lists, state). Godoc on every exported symbol.
- `gofmt`, `go vet ./...`, `go test ./...`, `go mod tidy` clean. Fix lint, don't disable.

### Terraform Plugin Framework
- Native schema/model/validator/diagnostic/plan-modifier patterns. snake_case attribute names.
- Field-level validators for simple constraints; **config validators** for cross-field
  rules; plan modifiers for replace/computed/unknown/stable-state.
- Mark secrets sensitive (`ssh_key`, tokens, bgp_key). Make lifecycle explicit:
  CRUD + Import + refresh must be consistent. Any RequiresReplace must be intentional,
  documented, and tested.
- Diagnostics must state **what field failed, why, and how to fix it** with the attribute
  path. No vague "invalid configuration".
- No noisy diffs from map ordering / generated ids / default handling / unstable
  serialization. Import must be honest about what it can/can't reconstruct.
- Avoid breaking schema changes; deprecate before replacing when practical.

### Testing (prove behavior, not coverage)
- **Unit:** validators, model↔FIM conversion (both directions), null/unknown handling,
  invalid config, error paths, deterministic ordering, time parse/format, user-data
  encode/decode, catalog/resource-summary parsing, lifecycle helpers, import helpers.
- **Golden:** generated GraphML and any serialized topology where exact structure matters.
  Normalize nondeterministic values; readable fixtures; explicit `-update` to regenerate
  (`go test ./... -update`); never snapshot garbage.
- **Acceptance** (`TF_ACC=1`, gated on live FABRIC creds): provider config, data-source
  read, create/read/refresh-no-diff, update where supported, replacement where required,
  import where supported, destroy cleanup, and invalid-config diagnostics. Where live
  testing isn't possible, say so in a TODO with the reason.
- Every bug fixed in this effort gets a regression test. Don't delete hard tests or loosen
  assertions. Don't over-mock to the point the test no longer proves provider behavior.

### Errors / logging / secrets
- User config errors → diagnostics; provider bugs → clear internal errors; remote failures
  preserve the underlying cause. Never log/expose tokens, SSH private keys, certs, bgp_key.
  Retries (if any) must be explicit, bounded, documented. No raw API-response dumps except
  redacted and behind debug logging.

---

## 4. Coverage Matrix (target state)

| Capability | FABlib/FIM source | FIM (Go) today | Provider today | Target home | Workstream |
|---|---|---|---|---|---|
| Labels (all fields) | `capacities_labels.py:339`, `value_types.go:66` | ✅ full | ❌ none | reusable `labels` block | **A1** |
| Capacities explicit | `value_types.go:22` | ✅ | partial (cores/ram/disk) | extend node | A2 |
| CapacityHints (instance_type) | `options.go` | ✅ | ✅ | — | — |
| Boot script | `options.go` BootScript | ✅ | ❌ | `node.boot_script` | A2 |
| User-data routes/post-boot | `node.py:4003-4101` | ❌ (opaque blob) | ❌ | **new FIM typed env** | **A3/A4** |
| Sub-interfaces (VLAN) | `interface.py:1451`, `facades.go:403` | ✅ | ❌ | `interface.sub_interface` | A5 |
| Host placement | `node.py:858` | ✅ (label) | ❌ | `node.host` (sugar) | A1 |
| Storage component | `node.py:3884` | ✅ (catalog) | partial (type enum) | `node.storage` | A2 |
| L3 metadata (gateway/subnet/site/technology) | `network_service.py:372`, `options.go:71` | ✅ | ❌ | extend `network` | **B1** |
| L3VPN + L2Path/Multisite/VLAN/MPLS types | `enums.go:25` | ✅ | ❌ (OneOf missing) | extend enum | B1 |
| L2 auto-select | `network_service.py:141` | ❌ | ❌ | **new FIM helper** | **B2** |
| Facility ports | `slice.py:1250`, `topology.go:117` | ✅ | ❌ | `facility_port` block | **B3** |
| Mirror direction aliases | `slice.py:1053` | ✅ wire | ✅ wire | optional aliasing | B4 |
| Switch/P4 nodes | `slice.py:1488`, `topology.go:150` | ✅ | ❌ | `switch` block | B5 |
| FABRIC time format | `swagger.yaml` | ❌ | ❌ (RFC3339 only) | **new FIM time type** | **C1** |
| Multiple SSH keys | adapter ready | n/a | ❌ (single) | `ssh_keys` list | C2 |
| Typed sites data source | `resources_summary.py`/advertised GraphML | partial | ❌ | `fabric_sites` | **D1/D2** |
| Typed facility ports DS | advertised GraphML | ❌ | ❌ | `fabric_facility_ports` | D2 |
| Richer resources DS filters | `api_resources.go` | n/a | ❌ | extend `fabric_resources` | D1 |
| Slivers detail DS | `adapter.GetSlivers` | n/a | ❌ | `fabric_slivers` | D3 |
| POA operations | `api_poas.go` | n/a | ❌ | `fabric_poa` (action) | **E1** |
| Metrics | `api_metrics.go` | n/a | ❌ | `fabric_metrics` DS | E2 |
| Drift reconciliation | `pkg/diff` + Read | ✅ diff | partial (warn only) | structured detection | **F1** |

---

## 5. Phasing & Sequencing

Dependencies dictate order. Each phase ends with green `go test ./...`, `go vet`, lint, and
updated golden files.

```
Phase 0  Foundations:   C1 (FABRIC time), reusable labels FIM glue, test harness upgrades
Phase 1  Node depth:    A1 labels, A2 capacities/boot/storage, A5 sub-interfaces, host sugar
Phase 2  User data:     A3 (FIM typed user-data envelope), A4 (routes/post-boot blocks)
Phase 3  Networks:      B1 L3 metadata + types, B2 L2 auto-select (FIM helper), B4 aliases
Phase 4  Topology nodes:B3 facility ports, B5 switches
Phase 5  Identity/keys: C2 multiple ssh keys
Phase 6  Data sources:  D1 resources filters, D2 typed sites/facility-ports, D3 slivers
Phase 7  Actions:       E1 POA, E2 metrics
Phase 8  Reconcile:     F1 drift, docs/examples sweep, final hardening
```

Phases 1–4 each touch the `fabric_slice` schema; land them sequentially to keep diffs
reviewable and golden files stable. Phases 6–7 are additive (new data sources/resources)
and can parallelize once Phase 0 lands.

---

## 6. Workstreams — Detailed Specs

> Convention for each: **Where**, **FIM work**, **Provider schema** (HCL + framework
> types), **Mapping**, **Validation/diagnostics**, **Tests**, **Acceptance criteria**.

---

### A1 — Reusable `labels` object + `node.host` sugar

**Where:** new `internal/provider/labels.go`; touch `models.go`, `resource_slice_schema.go`,
`topology_builder.go`.

**FIM work:** none new — map to `sliver.Labels` (`pkg/sliver/value_types.go:66-89`).
Confirm a label-validation entry point is exported; if FIM validates only at sliver build
time, that is acceptable (provider surfaces the FIM error as a diagnostic).

**Provider schema:** a single reusable nested object attribute `labels` usable on `node`,
`node.component`, `network`, `network.interface`, and (later) `facility_port`. Define it
once via a shared `func labelsAttribute() schema.SingleNestedAttribute`. Fields (all
`Optional`, strings unless noted), 1:1 with `sliver.Labels`:

```
labels {
  vlan          = string   # 0–4096
  vlan_range    = string   # "lo-hi"
  inner_vlan    = string
  ipv4          = string
  ipv4_range    = string
  ipv4_subnet   = string   # CIDR
  ipv6          = string
  ipv6_range    = string
  ipv6_subnet   = string
  mac           = string
  asn           = string   # 1..2^32-1
  bgp_key       = string   # sensitive
  account_id    = string
  region        = string
  local_name    = string
  local_type    = string
  device_name   = string
  numa          = number   # -1..7 (Int64, pointer in FIM)
  bdf           = string
  usb_id        = string
  instance        = string
  instance_parent = string  # host pinning target
}
```

`node.host` is **sugar**: an optional top-level `node.host` string that sets
`labels.instance_parent`. If both `node.host` and `node.labels.instance_parent` are set and
differ, return a config-validator error pointing at both paths.

**Mapping:** add `labelsModel` Go struct (`types.String`/`types.Int64`), a
`func (m labelsModel) toFIM() (*sliver.Labels, diag.Diagnostics)`. Skip null/unknown
fields. `numa`: convert `Int64` → `*int`. Keep field order deterministic in any computed
echo-back.

**Validation/diagnostics:** field validators where cheap (vlan range, numa range, mac
regex via `stringvalidator.RegexMatches`). Cross-field: `bgp_key` requires `asn` (BGP
peering); diagnostic must name both attribute paths and explain.

**Tests:**
- Unit: `labels_test.go` — every field round-trips to `sliver.Labels`; null/unknown
  produce nil/zero; numa pointer handling; mac/vlan validator rejects bad input with the
  right path; `host` vs `instance_parent` conflict diagnostic.
- Golden: a `bare_vm` + node labels topology → GraphML golden (`testdata/`), proving labels
  serialize deterministically.

**Acceptance:** `terraform plan` with a labels block on a node produces a valid topology;
invalid vlan yields a precise diagnostic; `node.host` ends up as `instance_parent` in the
emitted GraphML.

---

### A2 — Explicit capacities, boot script, storage component

**Where:** `resource_slice_schema.go` (node attrs), `topology_builder.go`.

**FIM work:** none (Capacities, BootScript, ComponentTypeStorage/NVME already present).

**Provider schema (node additions):**
```
node {
  # existing: name, site, instance_type, image_ref, image_type, cores, ram, disk
  boot_script = string   # ≤1024 bytes -> NodeOpts.BootScript; validate length
  storage {              # 0+; sugar over add_component(type=Storage/NVME)
    name       = string  # required
    model      = string  # e.g. "NAS" (Storage) / "P4510" (NVME); default "NAS"
    auto_mount = bool     # -> user-data flag (see A3); default false
  }
}
```
Keep `cores/ram/disk` and `instance_type` mutually-aware: if `instance_type` is set, the
orchestrator derives capacities; emit a config-validator **warning** (not error) if both
`instance_type` and explicit `cores/ram/disk` are set, since FIM prefers the hint.

**Mapping:** `boot_script` → `NodeOpts.BootScript` (length-validated). `storage` blocks →
`Node.AddComponent` with the appropriate `ComponentType`. `auto_mount` is recorded in the
typed user-data envelope from A3 (so do A3 first or stub the flag).

**Tests:** unit for boot_script length validator; storage component → catalog lookup →
GraphML golden; warning when instance_type + explicit capacities coexist.

**Acceptance:** a node with a Storage component and boot script serializes; over-length
boot script is rejected with a clear diagnostic.

---

### A3 — FIM typed user-data envelope (`fablib_data`)

This is the foundation for routes/post-boot/storage flags. **It is FIM work**, because the
same envelope must be written by the provider and (eventually) read back for drift.

**Where:** new package `fabric-go-fim/pkg/userdata/` (`userdata.go`, `userdata_test.go`).

**FIM work:** define typed structs matching FABlib's `fablib_data` JSON
(`fablib/node.py:3935-4101`). Minimum:
```go
package userdata

type NodeData struct {
    Routes           []Route       `json:"routes,omitempty"`
    PostBootTasks    []PostBootTask `json:"post_boot_tasks,omitempty"`
    PostUpdate       []string      `json:"post_update_commands,omitempty"`
    Storage          *Storage      `json:"storage,omitempty"`
    // preserve unknown keys to avoid clobbering FABlib-written data on round-trip
    Extra map[string]json.RawMessage `json:"-"`
}
type Route struct { Subnet string `json:"subnet"`; NextHop string `json:"next_hop"` }
type PostBootTask struct {
    Type string   `json:"type"` // "upload_file" | "upload_directory" | "execute"
    Args []string `json:"args"`
}
type Storage struct { Enabled bool `json:"storage"`; Cluster string `json:"storage_cluster,omitempty"` }
```
Provide `Encode(NodeData) ([]byte, error)` and `Decode([]byte) (NodeData, error)` with:
- Deterministic key ordering (sorted) so GraphML is stable.
- Unknown-key preservation (so we never destroy data FABlib wrote).
- ≤2048-byte enforcement matching FIM's `UserData` cap, returning a typed error.

Wire it into `NodeOpts.UserData` (the existing `[]byte`) so callers set typed data, not raw
bytes. Add a `NodeOpts.NodeData *userdata.NodeData` convenience or document that callers
must `Encode` themselves — pick one and be consistent; recommend a helper on `NodeOpts`.

**Tests (FIM):** encode/decode round-trip; unknown-key preservation; size cap error;
deterministic ordering (golden JSON).

**Acceptance:** FIM can produce a node whose `UserData` GraphML property contains a stable,
FABlib-compatible JSON envelope.

---

### A4 — Provider `route` and `post_boot` blocks

**Where:** `resource_slice_schema.go`, `topology_builder.go`, new conversion in
`internal/provider/userdata.go`.

**Boundary note (document in schema description):** these blocks *declare* configuration
into the slice's user-data. The provider does **not** SSH in and execute them; FABlib (or a
user's own tooling) consumes the envelope post-boot. State this honestly in docs.

**Provider schema (node additions):**
```
node {
  route {                # 0+
    subnet   = string    # CIDR, required
    next_hop = string    # IP or network name, required
  }
  post_boot_upload { local_path = string  remote_path = string }   # 0+
  post_boot_execute = list(string)        # commands
  post_update       = list(string)        # commands
}
```

**Mapping:** assemble a `userdata.NodeData`, `Encode`, set on `NodeOpts`. Validate CIDR and
IP with `stringvalidator`/parsing; diagnostics name the offending `route[N].subnet` path.

**Tests:** unit conversion model→`NodeData`; invalid CIDR diagnostic; golden GraphML with
routes + post-boot. Round-trip: state→envelope→decode equals input.

**Acceptance:** a node with routes and post-boot commands plans cleanly and serializes a
stable envelope; malformed CIDR is rejected with the attribute path.

---

### A5 — Sub-interfaces (VLAN child interfaces)

**Where:** `resource_slice_schema.go` (interface attrs), `topology_builder.go`
(`resolveInterface` path).

**FIM work:** none — `Interface.AddChildInterface` (`pkg/topology/facades.go:403`) needs
parent `DedicatedPort` + child VLAN; it validates uniqueness.

**Provider schema (network.interface additions):**
```
interface {
  # existing: node, component, port, name
  sub_interface {          # 0+; only valid on DedicatedPort-capable NICs
    name      = string     # required
    vlan      = string     # required, 0–4096
    bandwidth = number     # optional, Gbps -> capacity
    labels    = { ... }    # reuse A1 labelsAttribute()
  }
}
```

**Mapping:** after resolving the parent `*Interface`, call `AddChildInterface` per
sub-block. Surface FIM's parent-type/VLAN-uniqueness errors as diagnostics on the
`sub_interface[N]` path.

**Validation/diagnostics:** if the parent component model is not DedicatedPort-capable
(per FABlib only `ConnectX_5/6`), return a clear diagnostic instead of letting FIM fail
opaquely. Encode the capable-model set as a FIM-exported helper so the provider doesn't
hardcode FABRIC knowledge.

**Tests:** unit — sub-interface requires vlan (diagnostic); duplicate VLAN under one parent
rejected; golden `vm_subinterface` GraphML (FIM already has a fixture — mirror it).

**Acceptance:** a DedicatedPort NIC with two distinct-VLAN sub-interfaces serializes;
duplicate VLAN and non-capable parent each produce precise diagnostics.

---

### B1 — L3/L2 network metadata + full service-type set

**Where:** `resource_slice_schema.go` (network attrs + `type` OneOf), `topology_builder.go`.

**FIM work:** none — `NetworkServiceOpts` already has `Gateway`, `Subnet` (via Labels),
`Site`, `Technology`, `ERO`, `PathInfo`; enum already has all types.

**Provider schema (network additions / changes):**
```
network {
  # type: widen OneOf to include all FIM service types:
  #   L2Bridge, L2STS, L2PTP, L2Path, L2Multisite, VLAN, MPLS,
  #   FABNetv4, FABNetv6, FABNetv4Ext, FABNetv6Ext, L3VPN, PortMirror
  # (and make it Optional — see B2 auto-select)
  site       = string          # single-site service constraint
  technology = string          # e.g. "AL2S"
  subnet     = string          # CIDR -> service Labels.ipv4_subnet/ipv6_subnet
  gateway {                    # for FABNetv4/v6 / routed services
    ipv4        = string
    ipv4_subnet = string
    ipv6        = string
    ipv6_subnet = string
    mac         = string
  }
  labels = { ... }             # reuse A1; for asn/bgp_key/account_id/region peering
}
```

**Mapping:** populate `NetworkServiceOpts.{Site,Technology,Gateway,Labels}`; `subnet` →
appropriate Labels subnet field (choose v4/v6 by parsing). `gateway` must satisfy FIM's
"v4 pair XOR v6 pair" rule (`value_types.go:161-201`) — pre-validate and emit a diagnostic
naming the gateway sub-attributes.

**Validation/diagnostics:** config validator: `gateway`/`subnet` only meaningful for L3
types; `mirror_*` only for `PortMirror`; multi-site `site` conflicts. Each diagnostic
names the exact path and the valid combinations.

**Tests:** unit — L3VPN with gateway → opts; gateway v4+v6 both set rejected; golden
`fabnetv4`/`l3vpn` GraphML. Mirror fields on non-mirror type rejected.

**Acceptance:** an L3VPN and a FABNetv4 with gateway/subnet serialize; invalid gateway/
type combinations produce precise diagnostics.

---

### B2 — L2/L3 service-type auto-selection (FIM helper + optional `type`)

**Where:** new `fabric-go-fim/pkg/topology/infer.go` (+ `infer_test.go`); provider
`topology_builder.go`.

**FIM work (port of FABlib `__calculate_l2_nstype`, `network_service.py:141-199` and the
L3 mapping at `372-397`):** a pure function:
```go
// InferServiceType chooses the L2 service type from the connected interfaces.
// Mirrors FABlib: 0/1 site -> L2Bridge; 2 sites -> L2PTP unless BasicNIC present or
// facility-port/ERO constraints force L2STS; >2 sites is invalid for L2.
func InferServiceType(ifaces []*Interface, opts InferOpts) (sliver.ServiceType, error)
```
Inputs needed per interface: site, NIC model (BasicNIC vs others), interface type
(FacilityPort count), and an ERO flag. Encode the exact FABlib decision tree with comments
citing the Python source. Return a typed error for ">2 sites for L2" and similar.

Provider use: make `network.type` **Optional**. If omitted, call `InferServiceType` after
resolving interfaces; if FIM returns an error, surface it as a diagnostic on the `network`
block explaining why an explicit `type` is required. If `type` is set, honor it (and
optionally validate it against the inference for a warning on mismatch).

**Tests (FIM):** table-driven over FABlib's branches — single-site→L2Bridge;
2-site basic NIC→L2STS; 2-site dedicated→L2PTP; facility-port edge cases; >2 sites error.
These are the regression tests that prove parity with FABlib.

**Tests (provider):** omitted `type` infers correctly; ambiguous case → actionable
diagnostic; explicit `type` mismatch → warning.

**Acceptance:** a 2-node single-site L2 with no `type` plans as `L2Bridge`; a 2-site case
infers `L2STS`/`L2PTP` matching FABlib; impossible topology errors clearly.

---

### B3 — Facility ports

**Where:** `resource_slice_schema.go` (new top-level `facility_port` block), `models.go`,
`topology_builder.go`.

**FIM work:** none — `Topology.AddFacility` + `FacilityOpts` (`options.go:108-117`).

**Provider schema (new top-level block, list):**
```
facility_port {
  name       = string   # required
  site       = string   # required
  vlan       = string   # or list; FABlib accepts str|list -> one interface per vlan
  bandwidth  = number   # Gbps
  mtu        = number
  labels      = { ... }  # A1; ipv4/ipv6/mac/asn/bgp_key/account_id/region
  peer_labels = { ... }  # A1; AL2S peer side
  interface {            # optional explicit port configs; else one default
    name   = string
    vlan   = string
    labels = { ... }
  }
}
```

**Mapping:** build `FacilityOpts{Name,Site,Labels,Capacities{BW,MTU},Interfaces:[]FacilityInterfaceOpts}`.
`vlan` list → multiple `FacilityInterfaceOpts`. Then connect facility interfaces into
networks by name (a `network.interface` may reference a facility port). Define the
reference convention explicitly (e.g. `interface { facility = "<name>" }` as an alternative
to `node`).

**Validation:** site required; vlan format; at least one of vlan/interface present.
Diagnostics name `facility_port[N]` paths.

**Tests:** unit — single-vlan and multi-vlan facility → opts; golden `facility_port`
GraphML (FIM fixture exists). Provider: facility referenced by an L2PTP/L2STS network
serializes; FABlib "L2PTP needs ≥2 facility ports" rule surfaces via inference (B2).

**Acceptance:** a facility port attached to an L2 network plans and serializes; peer_labels
flow through for AL2S.

---

### B4 — Port-mirror direction aliases (optional, low priority)

**Where:** `resource_slice_schema.go`, `topology_builder.go`.

Keep canonical `Both`/`RX_Only`/`TX_Only` (already correct). Additionally accept
`both`/`rx`/`tx` and normalize via a plan modifier or in-builder mapping, so FABlib users
aren't surprised. Document both forms. This is a UX nicety, not a correctness fix.

**Tests:** unit — each alias maps to the correct `sliver.MirrorDirection`; canonical values
unchanged.

**Acceptance:** `mirror_direction = "rx"` and `"RX_Only"` produce identical topology.

---

### B5 — Switch / P4 nodes

**Where:** new top-level `switch` block; `models.go`, schema, builder.

**FIM work:** none — `Topology.AddSwitch` + `SwitchOpts` (`options.go:125-134`).

**Provider schema (new top-level block, list):**
```
switch {
  name        = string  # required
  site        = string  # required
  nports      = number  # default 8
  port_labels = { ... } # A1, applied to all ports
  # (port capacities optional)
}
```

**Mapping:** `SwitchOpts{Name,Site,NPorts,PortLabels}`. Switches are P4 by default; expose
`type` only if a second switch service type becomes relevant (otherwise omit — don't add
speculative config).

**Tests:** unit + golden `switch_node` GraphML (FIM fixture exists). Provider: switch with
custom nports serializes.

**Acceptance:** a P4 switch node with N ports plans and serializes.

---

### C1 — FABRIC time type (parse/format), fix lease handling

**Where:** new `fabric-go-fim/pkg/fabtime/` (`fabtime.go`, `fabtime_test.go`); provider
`resource_slice_schema.go` (validator), `resource_slice.go` (renew compute),
`fabricclient/adapter.go` (already passes strings).

**FIM work:** a small, well-tested time helper:
```go
package fabtime
// Layout is the orchestrator wire format documented in swagger:
//   "2023-01-01 16:20:15 +00:00"
const Layout = "2006-01-02 15:04:05 -07:00"
func Parse(s string) (time.Time, error)   // accepts Layout AND RFC3339
func Format(t time.Time) string           // emits Layout in UTC
```
Accept both inputs (so existing RFC3339 configs keep working) but **always emit `Layout`**
to the orchestrator. Confirm against a live orchestrator whether RFC3339 is also accepted;
if it is, this becomes belt-and-suspenders. If it is **not**, this fixes a real apply-time
bug — add a regression test reproducing the rejected-format case at the adapter boundary.

**Provider changes:**
- Replace the RFC3339-only regex validator on `lease_start_time` with a validator that
  accepts both forms (delegates to `fabtime.Parse`), with a diagnostic showing both
  accepted example formats.
- In Create/Update/renew, format outgoing lease times via `fabtime.Format` before calling
  the adapter (the renew path computes `now + lifetime_hours` at `resource_slice.go:~207`
  — route it through `fabtime.Format`).
- On Read, parse returned lease times via `fabtime.Parse` and store a **canonical**
  representation in state to avoid perpetual diffs (decide: store as RFC3339 UTC for
  Terraform-friendliness, or as `Layout`; document the choice and apply consistently in
  the data source too).

**Tests (FIM):** parse both layouts; format emits `Layout` in UTC; round-trip; reject
garbage. **Provider:** validator accepts both; outgoing value is `Layout`; Read→state is
canonical and produces no diff on re-plan (the regression test for noisy-diff).

**Acceptance:** a config using either time format applies; re-plan shows no lease diff;
the adapter sends the orchestrator-documented format.

---

### C2 — Multiple SSH keys

**Where:** `resource_slice_schema.go`, `models.go`, `resource_slice.go`, builder.

**FIM/client:** none — adapter already sends `[]string` (`adapter.go:55,60`).

**Provider schema:**
```
ssh_keys = list(string)   # sensitive; >=1
ssh_key  = string          # DEPRECATED alias for a single key; sensitive
```
Config validator: exactly one of `ssh_keys`/`ssh_key` set (or accept both and union, but
prefer the single-source rule with a clear deprecation message). `ssh_key_version` and the
RequiresReplace semantics carry over. Keep keys **out of state** as today
(`resource_slice.go:138` sets `SSHKey = Null`); do the same for `ssh_keys` (write-only) and
document that Read cannot reconstruct them (import honesty).

**Tests:** unit — `ssh_keys` list flows to adapter; `ssh_key` alias still works + emits
deprecation; both-set or neither-set diagnostics. Acceptance: create with two keys.

**Acceptance:** a slice with multiple keys applies; single `ssh_key` still works with a
deprecation notice.

---

### D1 — Richer `fabric_resources` data source

**Where:** `data_source_resources.go`; extend adapter `GetResources` (and add
`GetPortalResources`).

**Provider schema additions:** `start_date`, `end_date` (FABRIC time, validated via C1),
`includes`, `excludes` (string or list of site codes), `graph_format` (for the portal
variant). Keep `model` (opaque) for power users.

**Adapter:** extend to pass the unused query params (`api_resources.go` supports them); add
`PortalresourcesGet` wrapper.

**Tests:** unit on the adapter wiring (fake client asserts params); data-source read maps
fields. Acceptance: read with `includes="RENC,UKY"`.

**Acceptance:** the data source forwards date/site filters and returns the model.

---

### D2 — Typed `fabric_sites` and `fabric_facility_ports` data sources

This is the highest-value, highest-effort data work, because the orchestrator returns the
catalog as an **opaque GraphML/JSON `model` string** (`adapter.GetResources` →
`resp.Data[0].model`). Typed data sources require **decoding that GraphML into structured
Go** — which FIM does not do yet (FIM's `resources_summary.go` decodes the *portal JSON
summary*, a different shape).

**FIM work (new):** `fabric-go-fim/pkg/catalog/advertised.go`:
```go
// DecodeAdvertised parses an orchestrator advertised-topology GraphML model into typed
// sites/hosts/components/facility-ports/links.
func DecodeAdvertised(model string) (*Advertised, error)
type Advertised struct {
    Sites         []Site
    FacilityPorts []FacilityPort
    Links         []Link
}
type Site struct {
    Name string
    Cores, RAM, Disk struct{ Capacity, Allocated, Available int }
    Hosts []Host
    Components map[string]ComponentAvail // model -> capacity/available
    PTP bool
    IPv4Management bool
}
// ... Host, FacilityPort{Name,Site,Switch,VLANRange,Bandwidth}, Link{Name,Layer,Sites,Capacity}
```
Reuse the existing internal GraphML reader (`internal/graphml`) — likely promote a typed
decode path. Add golden tests against a captured advertised-topology fixture (capture one
real `level=2` response and commit a redacted fixture under `testdata/`).

**Provider data sources:**
- `fabric_sites` — optional filters (`includes`/`excludes`/`name`), returns a list of typed
  site objects (capacities, available components, ptp, mgmt-ip). 
- `fabric_facility_ports` — typed list of facility ports (name, site, switch, vlan_range,
  bandwidth).

Both call `adapter.GetResources(level, force_refresh)` then `catalog.DecodeAdvertised`.

**Tests:** FIM golden decode of the fixture; provider data-source read maps each field; a
filter test (`includes`) narrows results. Acceptance gated on live FABRIC.

**Acceptance:** `data.fabric_sites` exposes typed capacity/availability that a slice config
can reference (e.g. pick a site with an available GPU); `data.fabric_facility_ports` lists
ports usable by B3.

**Honest TODO:** if the advertised GraphML schema proves richer/messier than the fixture,
decode the fields we can verify and leave the rest as `model` (documented), rather than
inventing structure. Note it in the data-source docs.

---

### D3 — `fabric_slivers` data source (per-node detail)

**Where:** new `data_source_slivers.go`; adapter `GetSlivers` already exists
(`adapter.go:265`).

Expose per-sliver `sliver_id`, `sliver_type`, `state`, `pending_state`, `join_state`,
`management_ip` (parsed from the sliver detail map), `graph_node_id`. Lets users wire
outputs without parsing the slice model. Consider also enriching the `fabric_slice`
resource's computed `nodes` map from slivers for accurate `management_ip`.

**Tests:** unit map from `Sliver` → state; acceptance read for a known slice.

**Acceptance:** management IPs and per-node states are queryable.

---

### E1 — `fabric_poa` action resource (POA operations)

**Where:** new `resource_poa.go`; extend adapter with POA wrappers over `api_poas.go`.

POA is an **operation**, not desired state. Model it as a resource whose Create performs the
op and Read polls POA status to terminal (`Success`/`Failed`), with Delete a no-op (or
documented as non-reversible). Supported `operation` enum: `cpuinfo`, `numainfo`, `cpupin`,
`numatune`, `reboot`, `addkey`, `removekey`, `rescan` (`swagger.yaml` POA section). Schema
mirrors `PoaPost`/`PoaPostData` (`vcpu_cpu_map`, `node_set`, `bdf`, `keys{key,comment}`).

Because POAs aren't idempotent desired-state, document loudly: re-applying re-runs the op;
a `triggers`-style map (like `null_resource`) controls re-execution. Keep it out of the
slice schema.

**Adapter:** `CreatePOA(ctx, sliverID, PoaPost) (Poa, error)`, `GetPOA(ctx, poaID)`,
`pollPOA` using the existing `poller` package pattern.

**Tests:** unit — request mapping per operation; poll-to-terminal; failure surfaces POA
`error`. Acceptance (live): a `reboot` POA reaches `Success`.

**Acceptance:** a POA resource runs an operation and reports terminal status; failures are
clear diagnostics, not silent.

---

### E2 — `fabric_metrics` data source

**Where:** new `data_source_metrics.go`; adapter wrapper over `MetricsOverviewGet`.

Response is untyped (`[]map[string]interface{}`). Expose `excluded_projects` filter and a
JSON `results` string (don't invent a schema for unstructured data). Small, read-only.

**Tests:** unit adapter wiring; acceptance read.

**Acceptance:** overview metrics are retrievable as JSON.

---

### F1 — Drift reconciliation (within CRUD limits)

**Where:** `resource_slice.go` Read path (drift currently `DiffTopologies` → warnings,
`~306-319`).

FABRIC's lifecycle makes true bidirectional reconciliation impossible (the orchestrator
mutates allocations/labels post-create), so be explicit and correct rather than pretend:
- Use FIM `pkg/diff` to compare **desired** (from state-derived topology) vs **actual**
  (orchestrator model), but classify diffs:
  - **Computed/allocation fields** (assigned IPs, MACs, VLANs, sliver ids, management ip):
    expected — fold into computed outputs, never a diff.
  - **User-intent fields** (counts, types, sites, capacities, labels the user set):
    genuine drift — surface as a diagnostic and, where the field is mutable, reflect in
    plan so `terraform plan` shows it.
- Document precisely which attributes are authoritative-from-config vs
  computed-from-FABRIC, so users understand what drift detection can and cannot do. This is
  a docs deliverable as much as code.

**FIM work:** ensure `pkg/diff` can filter/classify by field category (extend if needed,
with tests). Keep the categorization in FIM (it's FABRIC semantics), not the provider.

**Tests:** unit — allocation-only differences produce no drift; a changed user label
produces drift; golden diff classification. Acceptance: mutate nothing, re-plan, expect no
diff (the canonical "no perpetual diff" guarantee).

**Acceptance:** re-applying an unchanged slice yields an empty plan; real config drift is
reported with attribute paths and a clear explanation of FABRIC's reconciliation limits.

---

## 7. Cross-Cutting: Validators, Plan Modifiers, Diagnostics

- Centralize reusable validators in `internal/provider/validators/` (vlan, numa, mac, cidr,
  fabric-time, site-code). Each with unit tests and precise messages.
- Reusable schema fragments (`labelsAttribute()`, `gatewayAttribute()`,
  `timeoutsAttribute()`) live in one file to keep node/network/facility consistent.
- Config validators (resource-level `ConfigValidators()`): mirror-fields-only-on-PortMirror;
  gateway/subnet-only-on-L3; host-vs-instance_parent; ssh key exactly-one; instance_type
  vs explicit capacities warning.
- Every diagnostic: `summary` = what failed; `detail` = why + how to fix; attach the
  `path.Path`. Add a lint check or review checklist item forbidding bare "invalid
  configuration" strings.

---

## 8. Testing Strategy & Golden Files

- **FIM new packages** (`fabtime`, `userdata`, `topology/infer`, `catalog/advertised`):
  each ships unit + golden tests in the FIM repo. The L2 inference table is the parity
  proof against FABlib — comment each case with the Python `file:line` it mirrors.
- **Provider golden GraphML:** add `internal/provider/testdata/*.graphml` for every new
  schema capability (labels, sub-interface, facility port, switch, L3VPN, routes). Provide
  a `-update` flag; normalize the random GraphID (the builder uses `DeriveGraphID` — pin it
  in tests via a fixed seed or post-process).
- **Provider unit conversion tests:** model→FIM and FIM/orchestrator→state for each new
  block, including null/unknown.
- **Acceptance** (`make testacc`, gated on `TF_ACC` + live creds): one focused test per new
  capability plus the no-perpetual-diff regression. Where live testing is infeasible (e.g.
  POA reboot in CI), leave a documented TODO and cover the request-mapping with unit tests.
- Keep `make test` (unit + golden, no creds) green and fast as the default gate.

---

## 9. Documentation & Examples

- Regenerate provider docs (`make docs` / `tfplugindocs`) after each schema change; ensure
  every attribute has a `Description`/`MarkdownDescription`.
- `examples/` — runnable, minimal configs per capability: labels, facility port, switch,
  L3VPN with gateway, sub-interfaces, routes/post-boot, multi-key, typed `data.fabric_sites`
  driving a slice. Mark anything non-runnable as illustrative.
- README: document FABRIC creds/env (`FABRIC_TOKEN`, `FABRIC_TOKEN_LOCATION`,
  `orchestrator_url`, `credmgr_url`), permission tags, import limitations (ssh keys not
  reconstructable; user-data envelope honesty), which fields force replacement, deprecated
  fields (`ssh_key`), and the drift-reconciliation limits from F1.
- Remove stale examples; ensure every example var name is realistic.

---

## 10. Risks, Unknowns, Honest TODOs

- **Advertised-topology GraphML schema (D2)** is the biggest unknown — capture a real
  fixture early; decode only verified fields; leave the rest as `model`.
- **Lease-time format (C1)** — confirm empirically whether the orchestrator accepts RFC3339;
  the plan is safe either way (we emit the documented format), but the regression test's
  framing depends on the answer.
- **CephFS/storage runtime (A2/A3)** — the declarative part (Storage component + auto_mount
  flag in user-data) is in scope; the actual mount orchestration is FABlib runtime and is
  documented out-of-scope.
- **L3VPN/AL2S peering** — `peer_labels`/`technology` are plumbed, but real AL2S peering may
  need fields not yet surfaced; add as discovered, with tests, not speculatively.
- **POA idempotency (E1)** — operations aren't desired-state; the `triggers` pattern and
  loud docs are the mitigation.

---

## 11. Tooling & CI Hardening (do alongside Phase 0)

- Expand `.golangci.yml` beyond the current 4 linters: add `gofumpt`/`gci` (imports),
  `revive` or `gocritic`, `errorlint`, `bodyclose`, `nilerr`, `misspell`, `unparam`. Fix
  findings; justify any disables inline.
- `go mod tidy` in all three modules; keep `replace` directives.
- Ensure `make fmt` (gofmt + vet), `make lint`, `make test` are the PR gate; `make testacc`
  is the nightly/manual gate.
- Add a tiny CI workflow (if absent) running `fmt`/`vet`/`lint`/`test` on PRs.

---

## 12. Definition of Done (per workstream and overall)

A workstream is done when: schema + mapping + validators land; unit + golden tests prove
behavior (incl. null/unknown + error paths); acceptance test exists (or a documented TODO
explains why not); docs/examples updated; `make fmt lint test` green; no new FABRIC logic
leaked into the provider.

Overall done when the coverage matrix (§4) is satisfied or each unmet cell has an honest,
documented TODO with rationale.

---

## 13. Final Deliverable Summary

Implemented through Phase 8:
- `ssh_keys` was added as the preferred multi-key slice input while `ssh_key` remains
  a deprecated single-key alias. Both key inputs are sensitive and cleared from state
  after apply.
- `fabric_resources` now forwards FABRIC time filters, include/exclude site filters,
  and portal graph format requests.
- `fabric-go-fim/pkg/catalog` now decodes verified advertised-topology fields for
  typed `fabric_sites` and `fabric_facility_ports` data sources.
- `fabric_slivers` exposes per-sliver identity, state, pending/join state, management
  IP, and graph node ID.
- `fabric_poa` runs POA operations as non-reversible action resources, polls terminal
  status, and uses `triggers` to force re-execution.
- `fabric_metrics` returns the unstructured metrics overview as JSON.
- FIM drift comparison now classifies user-intent vs FABRIC-computed fields, and the
  provider reports only configuration-owned drift while folding allocation/runtime
  values into computed state.
- Provider docs, README, and examples were refreshed. `plan.md` and `PLAN2.md` now
  point to this file as the active plan.

Tests added:
- SSH key source validation, alias compatibility, and multi-key create flow.
- Resource query and portal-resource adapter/data-source wiring.
- Advertised-topology decode golden fixture plus typed site/facility-port mapping.
- Sliver adapter/data-source mapping.
- POA adapter mapping, poller behavior, resource create/failure/read behavior.
- Metrics adapter/data-source mapping.
- Drift classification and provider drift-warning behavior.

Tests intentionally not added:
- Live acceptance tests for multi-key slices, typed advertised data, slivers, POA reboot,
  and metrics require FABRIC credentials and suitable live resources; unit coverage was
  added and acceptance TODOs remain documented in the workstream checkpoints.

Remaining TODOs:
- Refresh the advertised-topology fixture from a live redacted `level=2` FABRIC response
  when credentials are available.
- Add live acceptance coverage for `fabric_poa`, `fabric_slivers`, `fabric_metrics`,
  and typed advertised data sources.
- Replace illustrative examples with runnable configs as facility-port attachment,
  switch, sub-interface, post-boot, and route schema support lands.

Breaking changes and deprecations:
- No behavior was intentionally removed. `ssh_key` is deprecated in favor of `ssh_keys`
  but remains supported.
- `fabric_poa` is additive and intentionally re-runs only when replacement-triggering
  arguments change.

Commands run during implementation:
- `go mod tidy` in `fabric-go-fim`, `fabric-orchestrator-go-client`, and
  `terraform-provider-fabric`.
- `go build ./...` in all three modules.
- `make test` in `terraform-provider-fabric` after each workstream.
- Final hardening commands are recorded in the Phase 8 checkpoint.

The end state must read as the work of a careful Go/Terraform provider engineer: clean
package boundaries (FABRIC logic in FIM, Terraform in the provider), precise diagnostics,
deterministic GraphML, honest import/drift docs, and tests that fail for real regressions.
