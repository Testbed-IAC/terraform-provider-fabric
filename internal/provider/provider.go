package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	fabdatasource "github.com/Testbed-IAC/terraform-provider-fabric/internal/datasource"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/poa"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	fabslice "github.com/Testbed-IAC/terraform-provider-fabric/internal/slice"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

const (
	defaultOrchestratorURL = "https://orchestrator.fabric-testbed.net"
	defaultCredmgrURL      = "https://cm.fabric-testbed.net"
)

type FabricProvider struct {
	version string
	client  fabricclient.API
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FabricProvider{version: version}
	}
}

func NewWithClient(version string, client fabricclient.API) func() provider.Provider {
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
				Description:         "FABRIC bearer JWT. May also be set with FABRIC_TOKEN.",
				MarkdownDescription: "FABRIC bearer JWT. May also be set with the `FABRIC_TOKEN` environment variable. Obtain one from https://portal.fabric-testbed.net.",
			},
			"token_file": schema.StringAttribute{
				Optional:            true,
				Description:         "Path to a FABRIC portal token JSON file. May also be set with FABRIC_TOKEN_LOCATION.",
				MarkdownDescription: "Path to a FABRIC portal token JSON file. This supports automatic refresh using the file's `refresh_token`.",
			},
			"orchestrator_url": schema.StringAttribute{
				Optional:            true,
				Description:         "FABRIC orchestrator base URL.",
				MarkdownDescription: "FABRIC orchestrator base URL. Defaults to `" + defaultOrchestratorURL + "`.",
			},
			"credmgr_url": schema.StringAttribute{
				Optional:            true,
				Description:         "FABRIC credential manager base URL.",
				MarkdownDescription: "FABRIC credential manager base URL. Defaults to `" + defaultCredmgrURL + "`.",
			},
		},
	}
}

func (p *FabricProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "token", "ssh_key", "ssh_keys", "id_token", "refresh_token")
	ctx = tflog.MaskAllFieldValuesRegexes(ctx, regexp.MustCompile(`eyJ[A-Za-z0-9._\-]+`))

	var config providercfg.Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orchestratorURL := tfutil.DefaultString(tfutil.StringValue(config.OrchestratorURL), defaultOrchestratorURL)
	credmgrURL := tfutil.DefaultString(tfutil.StringValue(config.CredmgrURL), defaultCredmgrURL)
	tokenSource, authPath, err := resolveTokenSource(ctx, config, credmgrURL)
	if err != nil {
		tflog.Error(ctx, "provider authentication configuration failed", map[string]any{"error": err.Error()})
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"FABRIC Authentication Configuration Failed",
			err.Error(),
		)
		return
	}

	client := p.client
	if client == nil {
		client = fabricclient.New(orchestratorURL, tokenSource)
	}

	if _, err := client.GetResources(ctx, fabricclient.ResourcesQuery{Level: 1}); err != nil {
		switch {
		case errors.Is(err, fabricclient.ErrUnauthorized):
			tflog.Error(ctx, "FABRIC authentication preflight failed", map[string]any{"error": err.Error()})
			resp.Diagnostics.AddAttributeError(
				path.Root(authPath),
				"FABRIC Authentication Failed",
				"The orchestrator rejected the token with 401 Unauthorized. Refresh the token from https://portal.fabric-testbed.net and retry.",
			)
			return
		case errors.Is(err, fabricclient.ErrForbidden):
			project := tokenSource.Claims().Project()
			tflog.Error(ctx, "FABRIC authorization preflight failed", map[string]any{"error": err.Error(), "project_name": project.Name})
			resp.Diagnostics.AddError(
				"FABRIC Authorization Failed",
				fmt.Sprintf("The orchestrator rejected the request with 403 Forbidden. Confirm project %q is active and that your token belongs to that project.", project.Name),
			)
			return
		default:
			resp.Diagnostics.AddWarning(
				"FABRIC Connectivity Check Failed",
				fmt.Sprintf("Could not reach the FABRIC orchestrator at %s: %s. Terraform will continue, but apply operations may fail.", orchestratorURL, err.Error()),
			)
		}
	}

	data := &providercfg.Data{
		Client:          client,
		TokenSource:     tokenSource,
		ResourcesSource: catalog.NewResourcesClient("", http.DefaultClient),
	}
	resp.ResourceData = data
	resp.DataSourceData = data
	tflog.Info(ctx, "configured FABRIC provider", map[string]any{
		"orchestrator_url": orchestratorURL,
		"credmgr_url":      credmgrURL,
		"project_name":     tokenSource.Claims().Project().Name,
	})
}

func resolveTokenSource(ctx context.Context, config providercfg.Model, credmgrURL string) (auth.TokenSource, string, error) {
	token := tfutil.StringValue(config.Token)
	tokenFile := tfutil.StringValue(config.TokenFile)
	if token != "" && tokenFile != "" {
		return nil, "token", errors.New("configure exactly one of token or token_file, not both")
	}
	if tokenFile != "" {
		ts, err := auth.NewFileToken(expandPath(tokenFile), credmgrURL, http.DefaultClient)
		if err != nil {
			return nil, "token_file", err
		}
		return ts, "token_file", nil
	}
	if token != "" {
		ts, err := auth.NewStaticToken(token)
		if err != nil {
			return nil, "token", err
		}
		_, _ = ts.IDToken(ctx)
		return ts, "token", nil
	}
	if envPath := os.Getenv("FABRIC_TOKEN_LOCATION"); envPath != "" {
		ts, err := auth.NewFileToken(expandPath(envPath), credmgrURL, http.DefaultClient)
		if err != nil {
			return nil, "token_file", fmt.Errorf("reading FABRIC_TOKEN_LOCATION: %w", err)
		}
		return ts, "token_file", nil
	}
	for _, candidate := range defaultTokenLocations() {
		if _, err := os.Stat(candidate); err == nil {
			ts, err := auth.NewFileToken(candidate, credmgrURL, http.DefaultClient)
			if err != nil {
				return nil, "token_file", err
			}
			return ts, "token_file", nil
		}
	}
	if envToken := os.Getenv("FABRIC_TOKEN"); envToken != "" {
		ts, err := auth.NewStaticToken(envToken)
		if err != nil {
			return nil, "token", fmt.Errorf("parsing FABRIC_TOKEN: %w", err)
		}
		_, _ = ts.IDToken(ctx)
		return ts, "token", nil
	}
	return nil, "token_file", errors.New("set token_file, set FABRIC_TOKEN_LOCATION, place a token at ~/.fabric/token.json or ~/work/fabric_config/id_token.json, or set FABRIC_TOKEN. Get a token from https://portal.fabric-testbed.net")
}

func defaultTokenLocations() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".fabric", "token.json"),
		filepath.Join(home, "work", "fabric_config", "id_token.json"),
	}
}

func expandPath(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func (p *FabricProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		fabslice.NewResource,
		poa.NewResource,
	}
}

func (p *FabricProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		fabdatasource.NewSlice,
		fabdatasource.NewResources,
		fabdatasource.NewSites,
		fabdatasource.NewFacilityPorts,
		fabdatasource.NewSlivers,
		fabdatasource.NewMetrics,
	}
}

var _ provider.Provider = &FabricProvider{}
