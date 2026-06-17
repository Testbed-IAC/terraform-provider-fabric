package slice

import "time"

// FABRIC slice lifecycle states reported by the orchestrator. The provider keys
// terminal-state detection off these strings — the poller target/failure lists in
// Create/Update/Delete and the state switch in Read — so they are defined once
// here. A typo in an inline literal would silently change which states are
// treated as terminal; referencing these constants makes that impossible.
const (
	stateStableOK       = "StableOK"
	stateStableError    = "StableError"
	stateAllocatedError = "AllocatedError"
	stateModifyOK       = "ModifyOK"
	stateModifyError    = "ModifyError"
	stateClosing        = "Closing"
	stateClosingError   = "ClosingError"
	stateDead           = "Dead"
)

// Slice lifecycle timing defaults, shared by Create, Update, and Delete.
// slicePollInterval is the production state-polling cadence; acceptance tests
// shorten it through tfutil.PollInterval (FABRIC_POLL_INTERVAL).
// defaultLifecycleTimeout bounds each wait when the configuration sets no
// explicit timeouts block.
const (
	slicePollInterval       = 15 * time.Second
	defaultLifecycleTimeout = 30 * time.Minute
)

// managementIPWaitBudget bounds the extra ASM poll performed after a slice
// reaches a stable state. The orchestrator can report StableOK/ModifyOK a moment
// before the AM-assigned management IP is reflected in the per-node ASM, which
// would otherwise leave nodes.<name>.management_ip intermittently empty in fresh
// state (and flake StandardSliceGraphChecks). The wait returns as soon as every
// VM node carries an IP, so this ceiling is only reached when an IP never
// appears, in which case the provider proceeds with the empty value as before.
const managementIPWaitBudget = 2 * time.Minute
