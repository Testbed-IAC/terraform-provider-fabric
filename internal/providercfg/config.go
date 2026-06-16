// Package providercfg holds the provider-level configuration model and the
// dependencies (FABRIC client, token source, resources-summary source) shared with
// the provider's resources and data sources through Configure.
package providercfg

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

// Data contains provider dependencies shared with resources and data sources.
type Data struct {
	Client          fabricclient.API
	TokenSource     auth.TokenSource
	ResourcesSource ResourcesSummarySource
}

// FromProviderData extracts the shared *Data from a Configure request's
// ProviderData. Every resource and data source needs the same three-way result,
// so the logic lives here once instead of being copied into each Configure
// method: (nil, no diagnostics) before the provider is configured (the framework
// calls Configure with a nil ProviderData during early graph walks), (nil, an
// error diagnostic) when the value is the wrong type, or (the typed Data, no
// diagnostics) on success. Callers append the diagnostics and return when the
// result is nil.
func FromProviderData(providerData any) (*Data, diag.Diagnostics) {
	var diags diag.Diagnostics
	if providerData == nil {
		return nil, diags
	}
	data, ok := providerData.(*Data)
	if !ok {
		diags.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return nil, diags
	}
	return data, diags
}

// ResourcesSummarySource reads FABRIC portal resource summaries.
type ResourcesSummarySource interface {
	GetResourcesSummary(ctx context.Context, opts catalog.ResourcesOptions) (*catalog.ResourcesSummary, error)
}

// Model contains provider-level configuration.
type Model struct {
	Token           types.String `tfsdk:"token"`
	TokenFile       types.String `tfsdk:"token_file"`
	OrchestratorURL types.String `tfsdk:"orchestrator_url"`
	CredmgrURL      types.String `tfsdk:"credmgr_url"`
}
