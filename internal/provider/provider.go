package provider

import (
	"context"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

type FabricProvider struct {
	version string
	client  fabricclient.FabricClient
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FabricProvider{version: version}
	}
}

func NewWithClient(version string, client fabricclient.FabricClient) func() provider.Provider {
	return func() provider.Provider {
		return &FabricProvider{version: version, client: client}
	}
}

func (p *FabricProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fabric"
	resp.Version = p.version
}

func (p *FabricProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for the FABRIC testbed research network.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "FABRIC bearer token. May also be set with FABRIC_TOKEN.",
			},
			"orchestrator_url": schema.StringAttribute{
				Optional:    true,
				Description: "FABRIC orchestrator base URL. May also be set with FABRIC_ORCHESTRATOR_URL.",
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "FABRIC project UUID. May also be set with FABRIC_PROJECT_ID.",
			},
			"project_tags": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Project permission tags available to this token.",
			},
		},
	}
}

func (p *FabricProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "token", "ssh_key")
	ctx = tflog.MaskAllFieldValuesRegexes(ctx, regexp.MustCompile(`Bearer [A-Za-z0-9._\-]+`))

	var config FabricProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := stringValue(config.Token)
	if token == "" {
		token = os.Getenv("FABRIC_TOKEN")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing FABRIC token",
			"Set the token attribute or FABRIC_TOKEN environment variable.",
		)
		return
	}

	orchestratorURL := stringValue(config.OrchestratorURL)
	if orchestratorURL == "" {
		orchestratorURL = os.Getenv("FABRIC_ORCHESTRATOR_URL")
	}
	if orchestratorURL == "" {
		orchestratorURL = "https://orchestrator.fabric-testbed.net"
	}

	tags := map[string]bool{}
	var tagValues []string
	resp.Diagnostics.Append(config.ProjectTags.ElementsAs(ctx, &tagValues, false)...)
	for _, tag := range tagValues {
		tags[tag] = true
	}

	client := p.client
	if client == nil {
		client = fabricclient.New(orchestratorURL, token)
	}
	data := &providerData{client: client, projectTags: tags}
	resp.ResourceData = data
	resp.DataSourceData = data
	tflog.Info(ctx, "configured FABRIC provider", map[string]any{"orchestrator_url": orchestratorURL})
}

func (p *FabricProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSliceResource,
	}
}

func (p *FabricProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSliceDataSource,
		NewResourcesDataSource,
	}
}

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
