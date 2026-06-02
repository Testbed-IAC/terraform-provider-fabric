package slice

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
)

type testResourcesSource struct {
	summary *catalog.ResourcesSummary
	err     error
}

func (s testResourcesSource) GetResourcesSummary(context.Context, catalog.ResourcesOptions) (*catalog.ResourcesSummary, error) {
	return s.summary, s.err
}

func TestValidateResourcesSummaryUnknownSite(t *testing.T) {
	t.Parallel()
	var diags diag.Diagnostics
	validateResourcesSummary(context.Background(), bareVMModel(), testResourcesSource{summary: resourceSummary("UKY")}, &diags)
	if !diags.HasError() {
		t.Fatalf("expected unknown site diagnostic")
	}
	if !strings.Contains(diags.Errors()[0].Detail(), "RENC") {
		t.Fatalf("diagnostic detail = %q", diags.Errors()[0].Detail())
	}
}

func TestValidateResourcesSummaryInactiveSite(t *testing.T) {
	t.Parallel()
	var diags diag.Diagnostics
	summary := resourceSummary("RENC")
	summary.Data[0].Sites[0].State = "Maint"
	validateResourcesSummary(context.Background(), bareVMModel(), testResourcesSource{summary: summary}, &diags)
	if !diags.HasError() {
		t.Fatalf("expected inactive site diagnostic")
	}
	if !strings.Contains(diags.Errors()[0].Detail(), "Maint") {
		t.Fatalf("diagnostic detail = %q", diags.Errors()[0].Detail())
	}
}

func TestValidateResourcesSummaryCapacityWarnings(t *testing.T) {
	t.Parallel()
	var diags diag.Diagnostics
	model := bareVMModel()
	model.Nodes[0].Cores = types.Int64Value(20)
	model.Nodes[0].RAM = types.Int64Value(20)
	model.Nodes[0].Disk = types.Int64Value(20)
	summary := resourceSummary("RENC")
	summary.Data[0].Sites[0].CoresAvailable = 10
	summary.Data[0].Sites[0].RAMAvailable = 10
	summary.Data[0].Sites[0].DiskAvailable = 10
	validateResourcesSummary(context.Background(), model, testResourcesSource{summary: summary}, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %v", diags.Errors())
	}
	if len(diags.Warnings()) == 0 {
		t.Fatalf("expected availability warning")
	}
}

func TestValidateResourcesSummaryMissingComponent(t *testing.T) {
	t.Parallel()
	var diags diag.Diagnostics
	model := sharedNICModel()
	summary := resourceSummary("RENC")
	summary.Data[0].Sites[0].Components = map[string]catalog.ComponentAvailability{}
	validateResourcesSummary(context.Background(), model, testResourcesSource{summary: summary}, &diags)
	if !diags.HasError() {
		t.Fatalf("expected missing component diagnostic")
	}
}

func TestValidateResourcesSummaryFetchFailureWarns(t *testing.T) {
	t.Parallel()
	var diags diag.Diagnostics
	validateResourcesSummary(context.Background(), bareVMModel(), testResourcesSource{err: errors.New("network down")}, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %v", diags.Errors())
	}
	if len(diags.Warnings()) != 1 {
		t.Fatalf("warnings = %d, want 1", len(diags.Warnings()))
	}
}

func TestValidateTopologyCatchesNetworkServiceConstraints(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics
	validateTopology(context.Background(), crossSiteL2BridgeModel(), &diags)
	if !diags.HasError() {
		t.Fatalf("expected topology diagnostic")
	}
	if got := diags.Errors()[0].Summary(); got != "Invalid FABRIC topology" {
		t.Fatalf("summary = %q, want Invalid FABRIC topology", got)
	}
	if !strings.Contains(diags.Errors()[0].Detail(), "L2Bridge allows at most 1 sites") {
		t.Fatalf("diagnostic detail = %q", diags.Errors()[0].Detail())
	}
}

func resourceSummary(siteName string) *catalog.ResourcesSummary {
	return &catalog.ResourcesSummary{
		Data: []catalog.ResourcesSummaryData{{
			Sites: []catalog.SiteSummary{{
				Name:           siteName,
				State:          "Active",
				CoresCapacity:  100,
				CoresAvailable: 100,
				RAMCapacity:    100,
				RAMAvailable:   100,
				DiskCapacity:   100,
				DiskAvailable:  100,
				Components: map[string]catalog.ComponentAvailability{
					"SharedNIC-ConnectX-6": {Capacity: 2, Available: 2},
				},
			}},
		}},
	}
}
