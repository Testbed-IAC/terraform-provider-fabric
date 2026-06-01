package tfutil

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFabricTimeValidator(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "FABRIC layout", value: types.StringValue("2026-05-30 19:04:54 +00:00")},
		{name: "legacy orchestrator layout", value: types.StringValue("2026-05-30 19:04:54 +0000")},
		{name: "RFC3339", value: types.StringValue("2026-05-30T19:04:54Z")},
		{name: "invalid", value: types.StringValue("tomorrow"), wantError: true},
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := validator.StringRequest{
				Path:        path.Root("lease_start_time"),
				ConfigValue: tc.value,
			}
			var resp validator.StringResponse
			FabricTimeValidator{}.ValidateString(context.Background(), req, &resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Fatalf("HasError = %t, want %t: %v", got, tc.wantError, resp.Diagnostics)
			}
			if tc.wantError && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), FabricTimeExamples) {
				t.Fatalf("diagnostic detail = %q, want accepted examples", resp.Diagnostics.Errors()[0].Detail())
			}
		})
	}
}

func TestCanonicalFabricTimeValue(t *testing.T) {
	t.Parallel()
	got, err := CanonicalFabricTimeValue(types.StringValue("2026-05-30T19:04:54Z"))
	if err != nil {
		t.Fatalf("CanonicalFabricTimeValue: %v", err)
	}
	if want := "2026-05-30 19:04:54 +00:00"; got.ValueString() != want {
		t.Fatalf("CanonicalFabricTimeValue = %q, want %q", got.ValueString(), want)
	}
}
