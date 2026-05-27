package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

const defaultOrchestratorURL = "https://orchestrator.fabric-testbed.net"

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
				Optional:            true,
				Sensitive:           true,
				Description:         "FABRIC bearer token. May also be set with FABRIC_TOKEN.",
				MarkdownDescription: "FABRIC bearer JWT. May also be set with the `FABRIC_TOKEN` environment variable. Obtain one from https://portal.fabric-testbed.net → Experiments → Tokens.",
			},
			"orchestrator_url": schema.StringAttribute{
				Optional:            true,
				Description:         "FABRIC orchestrator base URL. May also be set with FABRIC_ORCHESTRATOR_URL.",
				MarkdownDescription: "FABRIC orchestrator base URL. May also be set with the `FABRIC_ORCHESTRATOR_URL` environment variable. Defaults to `" + defaultOrchestratorURL + "`.",
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Description:         "FABRIC project UUID. May also be set with FABRIC_PROJECT_ID.",
				MarkdownDescription: "FABRIC project UUID. May also be set with the `FABRIC_PROJECT_ID` environment variable. Find your project ID at https://portal.fabric-testbed.net → Projects.",
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

	projectID := stringValue(config.ProjectID)
	if projectID == "" {
		projectID = os.Getenv("FABRIC_PROJECT_ID")
	}

	orchestratorURL := stringValue(config.OrchestratorURL)
	if orchestratorURL == "" {
		orchestratorURL = os.Getenv("FABRIC_ORCHESTRATOR_URL")
	}
	if orchestratorURL == "" {
		orchestratorURL = defaultOrchestratorURL
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing FABRIC Token",
			"The provider requires a FABRIC bearer token. Set token in the provider "+
				"block or set the FABRIC_TOKEN environment variable. "+
				"Get a token from https://portal.fabric-testbed.net → Experiments → Tokens.",
		)
	}

	if projectID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("project_id"),
			"Missing FABRIC Project ID",
			"The provider requires a project_id. Set project_id in the provider block "+
				"or set the FABRIC_PROJECT_ID environment variable. "+
				"Find your project ID at https://portal.fabric-testbed.net → Projects.",
		)
	}

	if orchestratorURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("orchestrator_url"),
			"Missing Orchestrator URL",
			"Set orchestrator_url or FABRIC_ORCHESTRATOR_URL. "+
				"Default: "+defaultOrchestratorURL,
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if !strings.HasPrefix(token, "eyJ") {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Invalid FABRIC Token Format",
			"The token does not appear to be a valid JWT (expected to start with 'eyJ'). "+
				"FABRIC tokens are JWTs obtained from the portal. "+
				"The token may be expired or incorrectly copied.",
		)
		return
	}

	tags := map[string]bool{}
	var tagValues []string
	resp.Diagnostics.Append(config.ProjectTags.ElementsAs(ctx, &tagValues, false)...)
	for _, tag := range tagValues {
		tags[tag] = true
	}
	explicitTagCount := len(tags)

	// Auto-discover the token's project tags so permission.Validate can catch
	// policy violations at plan time without an orchestrator round-trip.
	// jwtTags is kept separate as the authoritative set for user-facing
	// error messages; tags (the union with explicit values) is used for the
	// plan-time check so existing configs that listed tags manually still work.
	jwtAuthoritative := map[string]bool{}
	jwtTags, jwtHasProject, jwtErr := jwtProjectTags(token, projectID)
	switch {
	case jwtErr != nil:
		tflog.Warn(ctx, "could not decode FABRIC token to read project tags", map[string]any{"err": jwtErr.Error()})
	case !jwtHasProject:
		resp.Diagnostics.AddAttributeWarning(
			path.Root("project_id"),
			"FABRIC Project Not In Token",
			fmt.Sprintf("project_id %q is not listed in the token's projects claim. "+
				"This means either (a) the token does not belong to that project, or "+
				"(b) you need to request a fresh token after joining the project. "+
				"Provider-side permission validation will be skipped — the orchestrator "+
				"will be the only check.",
				projectID),
		)
	default:
		for _, tag := range jwtTags {
			tags[tag] = true
			jwtAuthoritative[tag] = true
		}
		tflog.Info(ctx, "loaded FABRIC project tags from token", map[string]any{
			"project_id":    projectID,
			"tag_count":     len(jwtTags),
			"tags":          jwtTags,
			"explicit_tags": explicitTagCount,
		})

		// Warn about explicit tags the user listed that the token does NOT
		// actually grant — these will fool the local check but get rejected
		// by the orchestrator PDP.
		for _, tag := range tagValues {
			if !jwtAuthoritative[tag] {
				resp.Diagnostics.AddAttributeWarning(
					path.Root("project_tags"),
					"FABRIC Project Tag Not In Token",
					fmt.Sprintf("project_tags lists %q but your token does not actually grant "+
						"that tag for project %q. The orchestrator will reject any slice that "+
						"depends on it. Either remove %q from project_tags, or have a project "+
						"lead add the tag and then request a fresh token.",
						tag, projectID, tag),
				)
			}
		}
	}

	client := p.client
	if client == nil {
		client = fabricclient.New(orchestratorURL, token)
	}

	if _, err := client.GetResources(ctx, 1, false); err != nil {
		switch {
		case errors.Is(err, fabricclient.ErrUnauthorized):
			resp.Diagnostics.AddAttributeError(
				path.Root("token"),
				"FABRIC Authentication Failed",
				"The orchestrator rejected the token with 401 Unauthorized. "+
					"The token may be expired (FABRIC tokens are valid for ~1 hour). "+
					"Get a fresh token from https://portal.fabric-testbed.net → Experiments → Tokens.",
			)
			return
		case errors.Is(err, fabricclient.ErrForbidden):
			resp.Diagnostics.AddAttributeError(
				path.Root("project_id"),
				"FABRIC Authorization Failed",
				fmt.Sprintf("The orchestrator rejected the request with 403 Forbidden. "+
					"Verify that project_id %q is correct and that your token belongs "+
					"to a member of that project.",
					projectID),
			)
			return
		default:
			resp.Diagnostics.AddWarning(
				"FABRIC Connectivity Check Failed",
				fmt.Sprintf("Could not reach the FABRIC orchestrator at %s: %s. "+
					"Proceeding, but apply operations may fail.",
					orchestratorURL, err.Error()),
			)
		}
	}

	data := &providerData{
		client:         client,
		projectTags:    tags,
		jwtProjectTags: jwtAuthoritative,
		projectID:      projectID,
	}
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
