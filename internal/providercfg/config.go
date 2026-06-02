package providercfg

import (
	"context"

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
