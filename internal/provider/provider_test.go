package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_Provider_Schema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
	}{
		{name: "schema contains required configuration attributes"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &FabricProvider{version: "test"}
			var resp provider.SchemaResponse
			p.Schema(context.Background(), provider.SchemaRequest{}, &resp)
			for _, attr := range []string{"token", "orchestrator_url", "project_id"} {
				if _, ok := resp.Schema.Attributes[attr]; !ok {
					t.Fatalf("missing provider attribute %s", attr)
				}
			}
		})
	}
}

type configureValues struct {
	token           *string
	projectID       *string
	orchestratorURL *string
	projectTags     []string
}

func ptr(s string) *string { return &s }

// runConfigure builds a tfsdk.Config from v and invokes p.Configure.
func runConfigure(t *testing.T, ctx context.Context, p *FabricProvider, v configureValues) provider.ConfigureResponse {
	t.Helper()

	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	schema := schemaResp.Schema
	objectType := schema.Type().TerraformType(ctx).(tftypes.Object)

	strVal := func(p *string) tftypes.Value {
		if p == nil {
			return tftypes.NewValue(tftypes.String, nil)
		}
		return tftypes.NewValue(tftypes.String, *p)
	}

	tagsVal := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	if v.projectTags != nil {
		elems := make([]tftypes.Value, 0, len(v.projectTags))
		for _, tag := range v.projectTags {
			elems = append(elems, tftypes.NewValue(tftypes.String, tag))
		}
		tagsVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, elems)
	}

	raw := tftypes.NewValue(objectType, map[string]tftypes.Value{
		"token":            strVal(v.token),
		"project_id":       strVal(v.projectID),
		"orchestrator_url": strVal(v.orchestratorURL),
		"project_tags":     tagsVal,
	})

	req := provider.ConfigureRequest{
		TerraformVersion: "test",
		Config:           tfsdk.Config{Raw: raw, Schema: schema},
	}
	var resp provider.ConfigureResponse
	p.Configure(ctx, req, &resp)
	return resp
}

func clearFabricEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FABRIC_TOKEN", "")
	t.Setenv("FABRIC_PROJECT_ID", "")
	t.Setenv("FABRIC_ORCHESTRATOR_URL", "")
}

func containsDiagSummary(diags diag.Diagnostics, summary string) bool {
	for _, d := range diags {
		if d.Summary() == summary {
			return true
		}
	}
	return false
}

func diagSummaries(diags diag.Diagnostics) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Summary()+": "+d.Detail())
	}
	return out
}

func okFake() *fake.Client {
	return &fake.Client{
		ResourceFn: func(context.Context, int32, bool) (string, error) { return "", nil },
	}
}

// Tests below mutate FABRIC_* environment variables via t.Setenv, which forbids
// the use of t.Parallel anywhere up the stack. They are still table-driven and
// use t.Run.

func TestFabric_Provider_Configure_EmptyToken_Diagnostic(t *testing.T) {
	cases := []struct {
		name string
		v    configureValues
	}{
		{name: "all unset", v: configureValues{}},
		{name: "only project_id set", v: configureValues{projectID: ptr("project-1")}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			resp := runConfigure(t, context.Background(), p, tc.v)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error diagnostics, got %v", diagSummaries(resp.Diagnostics))
			}
			if !containsDiagSummary(resp.Diagnostics, "Missing FABRIC Token") {
				t.Fatalf("expected 'Missing FABRIC Token' diagnostic, got %v", diagSummaries(resp.Diagnostics))
			}
		})
	}
}

func TestFabric_Provider_Configure_InvalidTokenFormat_Diagnostic(t *testing.T) {
	cases := []struct {
		name  string
		token string
	}{
		{name: "plain string", token: "not-a-jwt"},
		{name: "wrong prefix", token: "abc.def.ghi"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:     ptr(tc.token),
				projectID: ptr("project-1"),
			})
			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error diagnostics, got %v", diagSummaries(resp.Diagnostics))
			}
			if !containsDiagSummary(resp.Diagnostics, "Invalid FABRIC Token Format") {
				t.Fatalf("expected 'Invalid FABRIC Token Format' diagnostic, got %v", diagSummaries(resp.Diagnostics))
			}
		})
	}
}

func TestFabric_Provider_Configure_EmptyProjectID_Diagnostic(t *testing.T) {
	cases := []struct {
		name string
		v    configureValues
	}{
		{name: "only token set", v: configureValues{token: ptr("eyJabc.def.ghi")}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			resp := runConfigure(t, context.Background(), p, tc.v)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error diagnostics, got %v", diagSummaries(resp.Diagnostics))
			}
			if !containsDiagSummary(resp.Diagnostics, "Missing FABRIC Project ID") {
				t.Fatalf("expected 'Missing FABRIC Project ID' diagnostic, got %v", diagSummaries(resp.Diagnostics))
			}
		})
	}
}

func TestFabric_Provider_Configure_EnvVarToken_Succeeds(t *testing.T) {
	cases := []struct {
		name string
		env  string
	}{
		{name: "valid jwt-like value", env: "eyJabc.def.ghi"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FABRIC_TOKEN", tc.env)
			t.Setenv("FABRIC_PROJECT_ID", "project-1")
			t.Setenv("FABRIC_ORCHESTRATOR_URL", "")
			p := &FabricProvider{version: "test", client: okFake()}
			resp := runConfigure(t, context.Background(), p, configureValues{})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", diagSummaries(resp.Diagnostics))
			}
			data, ok := resp.ResourceData.(*providerData)
			if !ok {
				t.Fatalf("ResourceData = %#v, want *providerData", resp.ResourceData)
			}
			if data.projectID != "project-1" {
				t.Fatalf("projectID = %q, want project-1", data.projectID)
			}
		})
	}
}

func TestFabric_Provider_Configure_EnvVarProjectID_Succeeds(t *testing.T) {
	cases := []struct {
		name string
		env  string
	}{
		{name: "uuid-shaped", env: "11111111-2222-3333-4444-555555555555"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FABRIC_TOKEN", "")
			t.Setenv("FABRIC_PROJECT_ID", tc.env)
			t.Setenv("FABRIC_ORCHESTRATOR_URL", "")
			p := &FabricProvider{version: "test", client: okFake()}
			resp := runConfigure(t, context.Background(), p, configureValues{
				token: ptr("eyJabc.def.ghi"),
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", diagSummaries(resp.Diagnostics))
			}
			data, ok := resp.ResourceData.(*providerData)
			if !ok {
				t.Fatalf("ResourceData = %#v, want *providerData", resp.ResourceData)
			}
			if data.projectID != tc.env {
				t.Fatalf("projectID = %q, want %q", data.projectID, tc.env)
			}
		})
	}
}

func TestFabric_Provider_Configure_DefaultOrchestratorURL(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "defaults when unset"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:     ptr("eyJabc.def.ghi"),
				projectID: ptr("project-1"),
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", diagSummaries(resp.Diagnostics))
			}
			if defaultOrchestratorURL == "" {
				t.Fatal("defaultOrchestratorURL constant is empty")
			}
		})
	}
}

func TestFabric_Provider_Configure_PreflightUnauthorized_Diagnostic(t *testing.T) {
	cases := []struct {
		name        string
		clientErr   error
		wantSummary string
	}{
		{name: "401 maps to auth failed", clientErr: fabricclient.ErrUnauthorized, wantSummary: "FABRIC Authentication Failed"},
		{name: "403 maps to authz failed", clientErr: fabricclient.ErrForbidden, wantSummary: "FABRIC Authorization Failed"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			client := &fake.Client{
				ResourceFn: func(context.Context, int32, bool) (string, error) { return "", tc.clientErr },
			}
			p := &FabricProvider{version: "test", client: client}
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:     ptr("eyJabc.def.ghi"),
				projectID: ptr("project-1"),
			})
			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error diagnostics, got %v", diagSummaries(resp.Diagnostics))
			}
			if !containsDiagSummary(resp.Diagnostics, tc.wantSummary) {
				t.Fatalf("expected %q diagnostic, got %v", tc.wantSummary, diagSummaries(resp.Diagnostics))
			}
		})
	}
}

func TestFabric_Provider_Configure_PreflightOtherError_Warning(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "network failure", err: errors.New("dial tcp: connection refused")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			client := &fake.Client{
				ResourceFn: func(context.Context, int32, bool) (string, error) { return "", tc.err },
			}
			p := &FabricProvider{version: "test", client: client}
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:     ptr("eyJabc.def.ghi"),
				projectID: ptr("project-1"),
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("expected warning, not error: %v", diagSummaries(resp.Diagnostics))
			}
			if !containsDiagSummary(resp.Diagnostics, "FABRIC Connectivity Check Failed") {
				t.Fatalf("expected connectivity warning, got %v", diagSummaries(resp.Diagnostics))
			}
		})
	}
}

func TestFabric_Provider_Configure_LoadsTagsFromJWT(t *testing.T) {
	cases := []struct {
		name      string
		projects  []jwtProject
		projectID string
		wantTags  []string
	}{
		{
			name: "project tags loaded",
			projects: []jwtProject{
				{UUID: "11111111-2222-3333-4444-555555555555", Tags: []string{"Slice.Multisite", "VM.NoLimitDisk"}},
			},
			projectID: "11111111-2222-3333-4444-555555555555",
			wantTags:  []string{"Slice.Multisite", "VM.NoLimitDisk"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			token := makeJWT(t, tc.projects)
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:     ptr(token),
				projectID: ptr(tc.projectID),
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", diagSummaries(resp.Diagnostics))
			}
			data, ok := resp.ResourceData.(*providerData)
			if !ok {
				t.Fatalf("ResourceData = %#v, want *providerData", resp.ResourceData)
			}
			for _, want := range tc.wantTags {
				if !data.projectTags[want] {
					t.Fatalf("projectTags missing %q; got %#v", want, data.projectTags)
				}
			}
		})
	}
}

func TestFabric_Provider_Configure_WarnsWhenProjectMissingFromJWT(t *testing.T) {
	cases := []struct {
		name      string
		projects  []jwtProject
		projectID string
	}{
		{
			name: "project not in token",
			projects: []jwtProject{
				{UUID: "11111111-2222-3333-4444-555555555555", Tags: []string{"A"}},
			},
			projectID: "99999999-9999-9999-9999-999999999999",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			token := makeJWT(t, tc.projects)
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:     ptr(token),
				projectID: ptr(tc.projectID),
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", diagSummaries(resp.Diagnostics))
			}
			if !containsDiagSummary(resp.Diagnostics, "FABRIC Project Not In Token") {
				t.Fatalf("expected 'FABRIC Project Not In Token' warning, got %v", diagSummaries(resp.Diagnostics))
			}
		})
	}
}

func TestFabric_Provider_Configure_ExplicitTagsMergedWithJWT(t *testing.T) {
	cases := []struct {
		name        string
		explicit    []string
		jwtTags     []string
		wantPresent []string
	}{
		{
			name:        "explicit and jwt unioned",
			explicit:    []string{"VM.NoLimitCPU"},
			jwtTags:     []string{"Slice.Multisite"},
			wantPresent: []string{"VM.NoLimitCPU", "Slice.Multisite"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearFabricEnv(t)
			p := &FabricProvider{version: "test", client: okFake()}
			projectID := "11111111-2222-3333-4444-555555555555"
			token := makeJWT(t, []jwtProject{{UUID: projectID, Tags: tc.jwtTags}})
			resp := runConfigure(t, context.Background(), p, configureValues{
				token:       ptr(token),
				projectID:   ptr(projectID),
				projectTags: tc.explicit,
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", diagSummaries(resp.Diagnostics))
			}
			data := resp.ResourceData.(*providerData)
			for _, want := range tc.wantPresent {
				if !data.projectTags[want] {
					t.Fatalf("projectTags missing %q; got %#v", want, data.projectTags)
				}
			}
		})
	}
}

func TestFabric_SliceResource_PDPDiagnostic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		err          error
		projectTags  map[string]bool
		wantSummary  string
		wantSubstrs  []string
		wantNotMatch bool
	}{
		{
			name: "pdp failure names missing tag",
			err: errors.New(
				`fabricclient: orchestrator internal server error (500): {"errors":[{"details":` +
					`"PDP Failure: PDP Authorization check failed - Policy Violation: ` +
					`Your project is lacking VM.NoLimitCPU or VM.NoLimit tag to provision VM with more than 2 cores."}]}`),
			projectTags: map[string]bool{"Slice.Multisite": true, "VM.NoLimitDisk": true},
			wantSummary: "FABRIC Policy Violation",
			wantSubstrs: []string{
				"VM.NoLimitCPU or VM.NoLimit",
				"Slice.Multisite",
				"VM.NoLimitDisk",
				"test-project",
				"portal.fabric-testbed.net",
			},
		},
		{
			name: "policy violation without parsed tag still helpful",
			err: errors.New(
				`fabricclient: orchestrator internal server error (500): "Policy Violation: something opaque"`),
			projectTags: map[string]bool{"VM.NoLimitDisk": true},
			wantSummary: "FABRIC Policy Violation",
			wantSubstrs: []string{"VM.NoLimitDisk", "test-project"},
		},
		{
			name: "policy violation with empty project tags falls back to none",
			err: errors.New(
				`fabricclient: orchestrator internal server error (500): "PDP Failure: opaque"`),
			projectTags: nil,
			wantSummary: "FABRIC Policy Violation",
			wantSubstrs: []string{"(none discovered from token)"},
		},
		{
			name:         "non-pdp error is not matched",
			err:          errors.New("kaboom"),
			projectTags:  nil,
			wantNotMatch: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &SliceResource{projectID: "test-project", jwtProjectTags: tc.projectTags}
			summary, detail, ok := r.pdpDiagnostic(tc.err)
			if tc.wantNotMatch {
				if ok {
					t.Fatalf("expected no match, got summary=%q detail=%q", summary, detail)
				}
				return
			}
			if !ok {
				t.Fatalf("expected pdpDiagnostic to match")
			}
			if summary != tc.wantSummary {
				t.Fatalf("summary = %q, want %q", summary, tc.wantSummary)
			}
			for _, sub := range tc.wantSubstrs {
				if !strings.Contains(detail, sub) {
					t.Fatalf("detail missing %q; full detail:\n%s", sub, detail)
				}
			}
		})
	}
}

func TestFabric_SliceResource_AddClientError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		err         error
		wantSummary string
		wantDetail  string
	}{
		{name: "unauthorized", err: fabricclient.ErrUnauthorized, wantSummary: "FABRIC Authentication Failed", wantDetail: "401"},
		{name: "forbidden", err: fabricclient.ErrForbidden, wantSummary: "FABRIC Authorization Failed", wantDetail: "403"},
		{name: "bad request", err: fabricclient.ErrBadRequest, wantSummary: "Invalid Slice Configuration", wantDetail: "GraphML"},
		{name: "server error", err: fabricclient.ErrServerError, wantSummary: "FABRIC Orchestrator Error", wantDetail: "500"},
		{name: "unknown error keeps default summary", err: errors.New("kaboom"), wantSummary: "Create FABRIC slice failed", wantDetail: "kaboom"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &SliceResource{projectID: "test-project"}
			diags := &diag.Diagnostics{}
			r.addClientError(diags, "Create FABRIC slice failed", tc.err)
			if !diags.HasError() {
				t.Fatalf("expected diagnostics to contain an error")
			}
			found := false
			for _, d := range *diags {
				if d.Summary() == tc.wantSummary && strings.Contains(d.Detail(), tc.wantDetail) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("did not find diag summary=%q detail~=%q in %v",
					tc.wantSummary, tc.wantDetail, diagSummaries(*diags))
			}
		})
	}
}
