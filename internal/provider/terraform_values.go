package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringValue(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

func int64Value(v types.Int64) int64 {
	if v.IsNull() || v.IsUnknown() {
		return 0
	}
	return v.ValueInt64()
}

func boolValue(v types.Bool) bool {
	if v.IsNull() || v.IsUnknown() {
		return false
	}
	return v.ValueBool()
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// stringSliceValue extracts a []string from a types.List of strings, returning
// nil for a null or unknown list. Null elements are dropped.
func stringSliceValue(ctx context.Context, v types.List) ([]string, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	var values []types.String
	if diags := v.ElementsAs(ctx, &values, false); diags.HasError() {
		return nil, diagnosticsError(diags)
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		out = append(out, value.ValueString())
	}
	return out, nil
}
