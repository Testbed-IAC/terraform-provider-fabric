package tfutil

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/fabtime"
)

// FabricTimeExamples contains accepted FABRIC time examples.
const FabricTimeExamples = fabtime.Examples

// FabricTimeValidator validates FABRIC orchestrator or RFC3339 timestamps.
type FabricTimeValidator struct{}

func (FabricTimeValidator) Description(context.Context) string {
	return "must be a FABRIC time or RFC3339 timestamp"
}

func (FabricTimeValidator) MarkdownDescription(context.Context) string {
	return "must be a FABRIC time or RFC3339 timestamp"
}

func (FabricTimeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	if _, err := fabtime.Parse(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid FABRIC lease time",
			fmt.Sprintf("The lease time must use the FABRIC orchestrator format (%s) or RFC3339. Original error: %s", FabricTimeExamples, err.Error()),
		)
	}
}

// CanonicalFabricTimeValue canonicalizes a Terraform string timestamp.
func CanonicalFabricTimeValue(value types.String) (types.String, error) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return value, nil
	}
	canonical, err := CanonicalFabricTimeString(value.ValueString())
	if err != nil {
		return value, err
	}
	return types.StringValue(canonical), nil
}

// CanonicalFabricTimeString canonicalizes a timestamp string.
func CanonicalFabricTimeString(value string) (string, error) {
	canonical, err := fabtime.Canonical(value)
	if err != nil {
		return "", err
	}
	return canonical, nil
}
