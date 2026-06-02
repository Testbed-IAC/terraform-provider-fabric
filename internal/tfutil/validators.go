package tfutil

import (
	"context"
	"net"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// CIDRStringValidator rejects values that are not valid CIDR notation.
type CIDRStringValidator struct{}

func (CIDRStringValidator) Description(context.Context) string {
	return "must be a valid CIDR subnet (e.g. 10.0.0.0/24)"
}

func (CIDRStringValidator) MarkdownDescription(context.Context) string {
	return "must be a valid CIDR subnet (e.g. `10.0.0.0/24`)"
}

func (CIDRStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	if _, _, err := net.ParseCIDR(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid CIDR subnet",
			"The value must be a valid CIDR subnet such as 10.0.0.0/24. Original error: "+err.Error(),
		)
	}
}

// IPStringValidator rejects values that are not valid IPv4 or IPv6 addresses.
type IPStringValidator struct{}

func (IPStringValidator) Description(context.Context) string {
	return "must be a valid IP address"
}

func (IPStringValidator) MarkdownDescription(context.Context) string {
	return "must be a valid IP address"
}

func (IPStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	if net.ParseIP(req.ConfigValue.ValueString()) == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP address",
			"The value must be a valid IPv4 or IPv6 address.",
		)
	}
}
