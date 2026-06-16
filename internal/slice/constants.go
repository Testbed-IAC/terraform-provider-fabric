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
