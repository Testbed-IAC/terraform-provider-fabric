package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func stringAttr(value string) types.String {
	return types.StringValue(value)
}

func providerTestJWT(t *testing.T, projectID string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(fabricclient.FabricClaims{
		Exp: time.Now().Add(time.Hour).Unix(),
		Projects: []fabricclient.FabricProject{{
			Name: "test-project",
			UUID: projectID,
			Tags: []string{"Slice.Multisite"},
		}},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	return header + "." + body + "." + sig
}

func TestFabric_Provider_Schema_NewAuthSurface(t *testing.T) {
	t.Parallel()
	p := &FabricProvider{version: "test"}
	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)
	for _, attr := range []string{"token", "token_file", "orchestrator_url", "credmgr_url"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Fatalf("missing provider attribute %s", attr)
		}
	}
	for _, removed := range []string{"project_id", "project_tags"} {
		if _, ok := resp.Schema.Attributes[removed]; ok {
			t.Fatalf("removed provider attribute %s is still present", removed)
		}
	}
}

func TestFabric_Provider_ResolveTokenSource_ExplicitToken(t *testing.T) {
	t.Parallel()
	ts, attr, err := resolveTokenSource(context.Background(), FabricProviderModel{
		Token: stringAttr(providerTestJWT(t, "project-1")),
	}, defaultCredmgrURL)
	if err != nil {
		t.Fatalf("resolveTokenSource returned error: %v", err)
	}
	if attr != "token" {
		t.Fatalf("attr = %q, want token", attr)
	}
	if ts.ProjectID() != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", ts.ProjectID())
	}
}

func TestFabric_Provider_ResolveTokenSource_TokenFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "id_token.json")
	body, err := json.Marshal(fabricclient.FabricTokenFile{
		IDToken:      providerTestJWT(t, "project-1"),
		RefreshToken: "refresh",
	})
	if err != nil {
		t.Fatalf("marshal token file: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	ts, attr, err := resolveTokenSource(context.Background(), FabricProviderModel{
		TokenFile: stringAttr(path),
	}, defaultCredmgrURL)
	if err != nil {
		t.Fatalf("resolveTokenSource returned error: %v", err)
	}
	if attr != "token_file" {
		t.Fatalf("attr = %q, want token_file", attr)
	}
	if ts.ProjectID() != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", ts.ProjectID())
	}
}

func TestFabric_Provider_ResolveTokenSource_BothExplicitErrors(t *testing.T) {
	t.Parallel()
	_, _, err := resolveTokenSource(context.Background(), FabricProviderModel{
		Token:     stringAttr(providerTestJWT(t, "project-1")),
		TokenFile: stringAttr("/tmp/token.json"),
	}, defaultCredmgrURL)
	if err == nil {
		t.Fatal("expected error when token and token_file are both set")
	}
}

func TestFabric_Provider_Configure_SetsProviderData(t *testing.T) {
	token := providerTestJWT(t, "project-1")
	p := &FabricProvider{version: "test", client: &fake.Client{
		ResourceFn: func(context.Context, int32, bool) (string, error) { return "", nil },
	}}
	resp := runProviderConfigure(t, p, map[string]tftypes.Value{
		"token":            tftypes.NewValue(tftypes.String, token),
		"token_file":       tftypes.NewValue(tftypes.String, nil),
		"orchestrator_url": tftypes.NewValue(tftypes.String, nil),
		"credmgr_url":      tftypes.NewValue(tftypes.String, nil),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure diagnostics: %v", resp.Diagnostics)
	}
	data, ok := resp.ResourceData.(*FabricProviderData)
	if !ok {
		t.Fatalf("ResourceData = %#v, want *FabricProviderData", resp.ResourceData)
	}
	if data.TokenSource.ProjectID() != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", data.TokenSource.ProjectID())
	}
}

func runProviderConfigure(t *testing.T, p *FabricProvider, values map[string]tftypes.Value) provider.ConfigureResponse {
	t.Helper()
	ctx := context.Background()
	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	req := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    tftypes.NewValue(objectType, values),
			Schema: schemaResp.Schema,
		},
	}
	var resp provider.ConfigureResponse
	p.Configure(ctx, req, &resp)
	return resp
}
