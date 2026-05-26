package testutil

import (
	"os"
	"testing"
)

func PreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{"FABRIC_TOKEN", "FABRIC_PROJECT_ID", "FABRIC_SSH_KEY"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}
