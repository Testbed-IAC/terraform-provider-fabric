package testutil

import (
	"os"
	"testing"
)

// PreCheck gates acceptance tests on TF_ACC=1. Testmode needs no FABRIC credentials,
// token files, or SSH keys — tokens are minted in-process and the substrate is mocked —
// so TF_ACC is the only required input (FABRIC_TESTMODE_URL is optional, defaulting to
// http://localhost:8700).
func PreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("acceptance tests require TF_ACC=1")
	}
}
