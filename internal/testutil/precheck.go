package testutil

import (
	"os"
	"testing"
)

func PreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("acceptance tests require TF_ACC=1")
	}
	if os.Getenv("FABRIC_TOKEN") == "" && os.Getenv("FABRIC_TOKEN_LOCATION") == "" {
		t.Fatal("FABRIC_TOKEN or FABRIC_TOKEN_LOCATION must be set for acceptance tests")
	}
	if os.Getenv("FABRIC_SSH_KEY") == "" {
		t.Fatal("FABRIC_SSH_KEY must be set for acceptance tests")
	}
}
