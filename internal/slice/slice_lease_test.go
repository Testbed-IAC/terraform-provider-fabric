// Tests for SliceResource.refreshFromSlice lease-time canonicalization: populating
// missing lease_start_time/lease_end_time from the orchestrator or from prior state,
// and canonicalizing user-configured RFC3339 values into the FABRIC '+00:00' layout.
package slice

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

func TestSliceResourceRefreshLeaseStartTime(t *testing.T) {
	t.Parallel()
	slice := &fabricclient.Slice{
		SliceID:        "slice-id",
		Name:           "slice",
		State:          "StableOK",
		GraphID:        "graph-id",
		LeaseStartTime: "2026-05-30 19:04:54 +0000",
		LeaseEndTime:   "2026-05-31 19:04:54 +0000",
	}
	const canonicalStart = "2026-05-30 19:04:54 +00:00"
	const canonicalEnd = "2026-05-31 19:04:54 +00:00"

	t.Run("populates missing lease start time", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		state := SliceResourceModel{LeaseStartTime: types.StringNull()}
		if err := (&SliceResource{}).refreshFromSlice(context.Background(), slice, &state, &diags); err != nil {
			t.Fatalf("refreshFromSlice: %v", err)
		}
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags.Errors())
		}
		if got := state.LeaseStartTime.ValueString(); got != canonicalStart {
			t.Fatalf("lease_start_time = %q, want %q", got, canonicalStart)
		}
		if got := state.LeaseEndTime.ValueString(); got != canonicalEnd {
			t.Fatalf("lease_end_time = %q, want %q", got, canonicalEnd)
		}
	})

	t.Run("canonicalizes configured lease start time", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		configured := "2026-05-30T19:04:54Z"
		state := SliceResourceModel{LeaseStartTime: types.StringValue(configured)}
		if err := (&SliceResource{}).refreshFromSlice(context.Background(), slice, &state, &diags); err != nil {
			t.Fatalf("refreshFromSlice: %v", err)
		}
		if diags.HasError() {
			t.Fatalf("diagnostics: %v", diags.Errors())
		}
		if got := state.LeaseStartTime.ValueString(); got != canonicalStart {
			t.Fatalf("lease_start_time = %q, want canonical value %q", got, canonicalStart)
		}
	})
}

// TestSliceResourceRefreshLeaseEndFromState covers the symmetric end-time fallback
// arm of refreshFromSlice: when the orchestrator omits lease_end_time, the value
// already in state is re-canonicalized (and an unparseable state value errors).
func TestSliceResourceRefreshLeaseEndFromState(t *testing.T) {
	t.Parallel()
	const canonicalEnd = "2026-05-31 19:04:54 +00:00"
	// The orchestrator returns a start time but no end time.
	partial := &fabricclient.Slice{
		SliceID:        "slice-id",
		Name:           "slice",
		State:          "StableOK",
		GraphID:        "graph-id",
		LeaseStartTime: "2026-05-30 19:04:54 +0000",
	}

	t.Run("populates lease end time from state", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		state := SliceResourceModel{LeaseEndTime: types.StringValue("2026-05-31T19:04:54Z")}
		if err := (&SliceResource{}).refreshFromSlice(context.Background(), partial, &state, &diags); err != nil {
			t.Fatalf("refreshFromSlice: %v", err)
		}
		if got := state.LeaseEndTime.ValueString(); got != canonicalEnd {
			t.Fatalf("lease_end_time = %q, want %q (from-state fallback)", got, canonicalEnd)
		}
	})

	t.Run("unparseable lease end time in state errors", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		state := SliceResourceModel{LeaseEndTime: types.StringValue("garbage")}
		if err := (&SliceResource{}).refreshFromSlice(context.Background(), partial, &state, &diags); err == nil {
			t.Fatal("expected error for an unparseable lease_end_time in state")
		}
	})
}
