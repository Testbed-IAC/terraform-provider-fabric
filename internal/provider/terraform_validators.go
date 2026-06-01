package provider

import (
	"context"
	"net"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// cidrStringValidator rejects values that are not valid CIDR notation.
type cidrStringValidator struct{}

func (cidrStringValidator) Description(context.Context) string {
	return "must be a valid CIDR subnet (e.g. 10.0.0.0/24)"
}

func (cidrStringValidator) MarkdownDescription(context.Context) string {
	return "must be a valid CIDR subnet (e.g. `10.0.0.0/24`)"
}

func (cidrStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
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

// ipStringValidator rejects values that are not valid IPv4 or IPv6 addresses.
type ipStringValidator struct{}

func (ipStringValidator) Description(context.Context) string {
	return "must be a valid IP address"
}

func (ipStringValidator) MarkdownDescription(context.Context) string {
	return "must be a valid IP address"
}

func (ipStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
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
