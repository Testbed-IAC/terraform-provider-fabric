package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

const (
	defaultCoreAPIURL  = "https://uis.fabric-testbed.net"
	defaultBastionHost = "bastion.fabric-testbed.net"
)

// BastionDataSource resolves the caller's FABRIC bastion login from the token,
// so configurations can SSH through the bastion without hardcoding it.
type BastionDataSource struct {
	tokenSource auth.TokenSource
	httpClient  *http.Client
}

type bastionDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	CoreAPIURL types.String `tfsdk:"core_api_url"`
	Host       types.String `tfsdk:"host"`
	Username   types.String `tfsdk:"username"`
	Email      types.String `tfsdk:"email"`
	UUID       types.String `tfsdk:"uuid"`
}

// NewBastion returns the FABRIC bastion data source.
func NewBastion() datasource.DataSource {
	return &BastionDataSource{}
}

func (d *BastionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bastion"
}

func (d *BastionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Resolve the caller's FABRIC bastion host and login from the configured token.",
		MarkdownDescription: "Resolve the caller's FABRIC bastion host and login from the configured token, via the FABRIC Core API. Use it to SSH to slice nodes through the bastion without hardcoding the username.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "The caller's FABRIC user UUID.",
				MarkdownDescription: "The caller's FABRIC user UUID.",
			},
			"core_api_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "FABRIC Core API base URL. Defaults to " + defaultCoreAPIURL + ".",
				MarkdownDescription: "FABRIC Core API base URL. Defaults to `" + defaultCoreAPIURL + "`.",
			},
			"host": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Bastion hostname. Defaults to " + defaultBastionHost + ".",
				MarkdownDescription: "Bastion hostname. Defaults to `" + defaultBastionHost + "`.",
			},
			"username": schema.StringAttribute{
				Computed:            true,
				Description:         "The caller's FABRIC bastion login.",
				MarkdownDescription: "The caller's FABRIC bastion login.",
			},
			"email": schema.StringAttribute{
				Computed:            true,
				Description:         "The caller's FABRIC account email.",
				MarkdownDescription: "The caller's FABRIC account email.",
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				Description:         "The caller's FABRIC user UUID.",
				MarkdownDescription: "The caller's FABRIC user UUID.",
			},
		},
	}
}

func (d *BastionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	data, diags := providercfg.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if data == nil {
		return
	}
	d.tokenSource = data.TokenSource
	d.httpClient = &http.Client{Timeout: 30 * time.Second}
}

func (d *BastionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config bastionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	coreURL := strings.TrimRight(tfutil.StringValue(config.CoreAPIURL), "/")
	if coreURL == "" {
		coreURL = defaultCoreAPIURL
	}
	host := tfutil.StringValue(config.Host)
	if host == "" {
		host = defaultBastionHost
	}

	token, err := d.tokenSource.IDToken(ctx)
	if err != nil {
		resp.Diagnostics.AddError("FABRIC token unavailable", err.Error())
		return
	}

	var who struct {
		Results []struct {
			UUID  string `json:"uuid"`
			Email string `json:"email"`
		} `json:"results"`
	}
	if err := d.get(ctx, coreURL+"/whoami", token, &who); err != nil {
		resp.Diagnostics.AddError("FABRIC whoami lookup failed", err.Error())
		return
	}
	if len(who.Results) == 0 {
		resp.Diagnostics.AddError("FABRIC whoami lookup failed", "the Core API returned no user record for this token")
		return
	}
	uuid := who.Results[0].UUID

	var person struct {
		Results []struct {
			BastionLogin string `json:"bastion_login"`
		} `json:"results"`
	}
	if err := d.get(ctx, coreURL+"/people/"+url.PathEscape(uuid)+"?as_self=true", token, &person); err != nil {
		resp.Diagnostics.AddError("FABRIC people lookup failed", err.Error())
		return
	}
	login := ""
	if len(person.Results) > 0 {
		login = person.Results[0].BastionLogin
	}
	if login == "" {
		resp.Diagnostics.AddError(
			"No bastion login for this account",
			"FABRIC has not assigned a bastion login to this user yet. Generate bastion keys on the FABRIC portal, then retry.",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, bastionDataSourceModel{
		ID:         types.StringValue(uuid),
		CoreAPIURL: types.StringValue(coreURL),
		Host:       types.StringValue(host),
		Username:   types.StringValue(login),
		Email:      types.StringValue(who.Results[0].Email),
		UUID:       types.StringValue(uuid),
	})...)
}

func (d *BastionDataSource) get(ctx context.Context, endpoint, token string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")
	res, err := d.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d: %s", endpoint, res.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}
