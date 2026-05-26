package provider

import (
	"os"
	"testing"
)

func TestAccFabric_Slice_BareVM(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func TestAccFabric_Slice_MultiSite(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func TestAccFabric_Slice_L2Bridge(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func TestAccFabric_Slice_UpdateDisk(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func TestAccFabric_Slice_Import(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func TestAccFabric_Slice_Disappears(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func TestAccFabric_Slice_NameReplaces(t *testing.T) {
	t.Parallel()
	requireAcceptance(t)
}

func requireAcceptance(t *testing.T) {
	t.Helper()
	if !acceptanceEnabled() {
		t.Skip("acceptance tests require TF_ACC=1, FABRIC_TOKEN, and FABRIC_PROJECT_ID")
	}
}

func acceptanceEnabled() bool {
	return os.Getenv("TF_ACC") == "1" && os.Getenv("FABRIC_TOKEN") != "" && os.Getenv("FABRIC_PROJECT_ID") != ""
}
