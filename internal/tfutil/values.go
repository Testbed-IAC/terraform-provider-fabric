// Package tfutil holds small, dependency-light helpers shared across the provider's
// resources and data sources: extracting Go values from terraform-plugin-framework
// types, FABRIC/RFC3339 time validation and canonicalization, CIDR/IP validators,
// and the acceptance-test poll-interval override.
package tfutil

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ErrInvalidDiagnostics is returned when framework diagnostics cannot be
// represented more specifically.
var ErrInvalidDiagnostics = errors.New("invalid diagnostics")

// StringValue extracts a string from a Terraform string value.
func StringValue(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

// Int64Value extracts an int64 from a Terraform int64 value.
func Int64Value(v types.Int64) int64 {
	if v.IsNull() || v.IsUnknown() {
		return 0
	}
	return v.ValueInt64()
}

// BoolValue extracts a bool from a Terraform bool value.
func BoolValue(v types.Bool) bool {
	if v.IsNull() || v.IsUnknown() {
		return false
	}
	return v.ValueBool()
}

// DefaultString returns fallback when value is empty.
func DefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// StringSliceValue extracts strings from a Terraform list of strings.
func StringSliceValue(ctx context.Context, v types.List) ([]string, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	var values []types.String
	if diags := v.ElementsAs(ctx, &values, false); diags.HasError() {
		return nil, DiagnosticsError(diags)
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

// StringValues extracts known string values from a Terraform string slice.
func StringValues(values []types.String) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		out = append(out, value.ValueString())
	}
	return out
}

// DiagnosticsError converts Terraform diagnostics into a wrapped Go error.
func DiagnosticsError(diags diag.Diagnostics) error {
	if !diags.HasError() {
		return nil
	}
	errs := make([]error, 0, len(diags.Errors()))
	for _, item := range diags.Errors() {
		errs = append(errs, fmt.Errorf("%w: %s: %s", ErrInvalidDiagnostics, item.Summary(), item.Detail()))
	}
	return errors.Join(errs...)
}
