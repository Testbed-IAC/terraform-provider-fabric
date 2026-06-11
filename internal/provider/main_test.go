package provider_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestMain gates acceptance tests on a reachable testmode stack. With TF_ACC unset it
// runs nothing here (acceptance tests self-skip via PreCheck); with TF_ACC=1 it waits
// for the orchestrator and for the one-shot `claim` service to populate the broker CBM
// before any slice create is attempted. No token setup — each test mints its own.
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC") != "1" {
		os.Exit(m.Run())
	}

	// The provider's plan-time site pre-check fetches the public FABRIC portal resources
	// summary (FABRIC_PORTAL_RESOURCES_URL, default the production portal). testmode does
	// not run the portal, and the production portal does not list the testmode site
	// (RENC), which would turn the check into a hard "Unknown FABRIC site" error. Point
	// it at an unreachable endpoint so the provider's documented soft-skip runs (a
	// warning, not an error). Site/capacity correctness is still enforced authoritatively
	// by the orchestrator/broker on apply (see ACCEPTANCE_TEST_PLAN.md A1). Tests do not
	// set this — it is internal to the testmode harness.
	if os.Getenv("FABRIC_PORTAL_RESOURCES_URL") == "" {
		_ = os.Setenv("FABRIC_PORTAL_RESOURCES_URL", "http://127.0.0.1:1/unreachable")
	}

	url := testutil.OrchestratorURL()
	if err := testutil.WaitForOrchestrator(url, 60*time.Second); err != nil {
		fmt.Fprintf(os.Stderr,
			"\nfabric testmode orchestrator not reachable: %v\n"+
				"start it from the ControlFramework repo:\n"+
				"  docker compose -f docker-compose-testmode.yaml up -d --build\n\n", err)
		os.Exit(1)
	}

	// The `claim` container is one-shot with no compose healthcheck; /resources has no
	// placeable compute until it merges the AM delegations. Require >=1 worker for the
	// single-site stack (3 RENC workers) and >=4 when multi-site (§4f) adds UKY's 3.
	minWorkers := 1
	if testutil.MultisiteEnabled() {
		minWorkers = 4
	}
	if err := testutil.WaitForResources(url, minWorkers, 120*time.Second); err != nil {
		fmt.Fprintf(os.Stderr,
			"\nfabric testmode broker resources not populated: %v\n"+
				"the one-shot claim service may not have run; re-run it:\n"+
				"  docker compose -f docker-compose-testmode.yaml up claim\n\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}
