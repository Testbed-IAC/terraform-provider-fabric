package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/fabtime"
)

const fabricTimeExamples = fabtime.Examples

type fabricTimeValidator struct{}

func (v fabricTimeValidator) Description(context.Context) string {
	return "must be a FABRIC time or RFC3339 timestamp"
}

func (v fabricTimeValidator) MarkdownDescription(context.Context) string {
	return "must be a FABRIC time or RFC3339 timestamp"
}

func (v fabricTimeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	_ = ctx
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	if _, err := fabtime.Parse(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid FABRIC lease time",
			fmt.Sprintf("The lease time must use the FABRIC orchestrator format (%s) or RFC3339. Original error: %s", fabricTimeExamples, err.Error()),
		)
	}
}

func canonicalFabricTimeValue(value types.String) (types.String, error) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return value, nil
	}
	canonical, err := canonicalFabricTimeString(value.ValueString())
	if err != nil {
		return value, err
	}
	return types.StringValue(canonical), nil
}

func canonicalFabricTimeString(value string) (string, error) {
	canonical, err := fabtime.Canonical(value)
	if err != nil {
		return "", err
	}
	return canonical, nil
}
