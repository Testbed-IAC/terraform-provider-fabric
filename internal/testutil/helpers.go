// Package testutil is the acceptance-test support library for the FABRIC provider:
// it mints testmode JWTs, builds provider/resource HCL, waits on the testmode stack,
// and verifies the orchestrator-returned ASM topology via the GraphML graph checks.
package testutil

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

// DefaultOrchestratorURL is the testmode orchestrator REST endpoint.
const DefaultOrchestratorURL = "http://localhost:8700"

// OrchestratorURL returns FABRIC_TESTMODE_URL or the testmode default.
func OrchestratorURL() string {
	if v := os.Getenv("FABRIC_TESTMODE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultOrchestratorURL
}

// SkipIfNoAcc skips a test unless TF_ACC=1. Use it in tests that hit the orchestrator;
// do NOT use it in plan-only permission tests, which must run in the fast unit job.
func SkipIfNoAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("acceptance test requires TF_ACC=1 and a running testmode stack")
	}
}

// MultisiteEnabled reports whether the second site AM (UKY) is expected to be up.
// The testmode stack runs site2-am by default, so multi-site is enabled unless
// explicitly disabled with FABRIC_TESTMODE_MULTISITE=0 (for a single-site stack).
func MultisiteEnabled() bool {
	return os.Getenv("FABRIC_TESTMODE_MULTISITE") != "0"
}

// SkipIfNoMultisite skips a test only when multi-site has been explicitly disabled.
func SkipIfNoMultisite(t *testing.T) {
	t.Helper()
	if !MultisiteEnabled() {
		t.Skip("multi-site disabled (FABRIC_TESTMODE_MULTISITE=0); needs the UKY site AM")
	}
}

// Client returns a FABRIC client pointed at the testmode orchestrator, authenticated
// with a freshly minted full-permission token. Used by CheckDestroy and out-of-band
// slice operations in acceptance tests.
func Client() (*fabricclient.Client, error) {
	ts, err := auth.NewStaticToken(FullToken())
	if err != nil {
		return nil, fmt.Errorf("building testmode token source: %w", err)
	}
	return fabricclient.New(OrchestratorURL(), ts), nil
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// UniqueName derives a slice name unique to a test (and OS process), so concurrent or
// repeated runs never collide on the shared TestProject. FABRIC slice names must be
// reasonably short and simple, so the test name is sanitized and truncated.
func UniqueName(t *testing.T) string {
	t.Helper()
	short := nonAlnum.ReplaceAllString(t.Name(), "-")
	short = strings.Trim(strings.ToLower(short), "-")
	if len(short) > 32 {
		short = short[:32]
	}
	return fmt.Sprintf("tf-acc-%s-%d", short, os.Getpid())
}
