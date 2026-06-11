package slice

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/permission"
)

// fakeTokenSource is a canned auth.TokenSource carrying a single project's tags. It
// lets the permission tests exercise the provider's real enforcement function
// (validatePermissionTags) without encoding a JWT — token encoding/decoding is verified
// separately in internal/testutil/token_test.go.
type fakeTokenSource struct {
	tags []string
}

func (f fakeTokenSource) IDToken(context.Context) (string, error) { return "", nil }
func (f fakeTokenSource) ProjectID() string                       { return "11111111-1111-1111-1111-111111111111" }
func (f fakeTokenSource) Claims() *auth.Claims {
	return &auth.Claims{Projects: []auth.Project{{
		Name: "TestProject",
		UUID: f.ProjectID(),
		Tags: f.tags,
	}}}
}

// allTags mirrors the full provider capability vocabulary so a "fully privileged"
// token can have exactly one tag removed per case.
var allTags = []string{
	permission.TagSliceNoLimitLifetime,
	permission.TagVMNoLimit, permission.TagVMNoLimitCPU, permission.TagVMNoLimitRAM, permission.TagVMNoLimitDisk,
	permission.TagSliceMultisite,
	permission.TagComponentGPU, permission.TagComponentFPGA, permission.TagComponentNVME, permission.TagComponentStorage,
	permission.TagComponentSmartNICConnectX5, permission.TagComponentSmartNICConnectX6,
	permission.TagComponentSmartNICConnectX7100, permission.TagComponentSmartNICConnectX7400,
	permission.TagComponentSmartNICBlueField2ConnectX6,
	permission.TagNetNoLimitBW, permission.TagNetFABNetv4Ext, permission.TagNetFABNetv6Ext, permission.TagNetPortMirroring,
}

func tagsExcept(remove string) []string {
	out := make([]string, 0, len(allTags))
	for _, t := range allTags {
		if t != remove {
			out = append(out, t)
		}
	}
	return out
}

// TestValidatePermissionTags_BlocksBeforeHTTP verifies the provider denies a slice at
// plan time (ModifyPlan) when the token lacks the capability tag the request implies.
// This is the genuine permission gate: it runs before any orchestrator call, and the
// testmode orchestrator would NOT enforce these tags anyway (PDP disabled — see
// ACCEPTANCE_TEST_PLAN.md C2).
func TestValidatePermissionTags_BlocksBeforeHTTP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		req     permission.Request
		tags    []string
		wantTag string // empty => expect no diagnostics
	}{
		{
			name:    "missing VM.NoLimitCPU for cores>2",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Cores: 8}}},
			tags:    tagsExcept(permission.TagVMNoLimitCPU),
			wantTag: permission.TagVMNoLimitCPU,
		},
		{
			name:    "missing VM.NoLimitDisk for disk>10",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Disk: 100}}},
			tags:    tagsExcept(permission.TagVMNoLimitDisk),
			wantTag: permission.TagVMNoLimitDisk,
		},
		{
			name:    "missing Component.GPU",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Components: []permission.Component{{Type: "GPU", Model: "RTX6000"}}}}},
			tags:    tagsExcept(permission.TagComponentGPU),
			wantTag: permission.TagComponentGPU,
		},
		{
			name:    "missing Component.FPGA",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Components: []permission.Component{{Type: "FPGA", Model: "Xilinx-U280"}}}}},
			tags:    tagsExcept(permission.TagComponentFPGA),
			wantTag: permission.TagComponentFPGA,
		},
		{
			name: "missing Slice.Multisite for two sites",
			req: permission.Request{Nodes: []permission.Node{
				{Name: "vm1", Site: "RENC"},
				{Name: "vm2", Site: "UKY"},
			}},
			tags:    tagsExcept(permission.TagSliceMultisite),
			wantTag: permission.TagSliceMultisite,
		},
		{
			name:    "missing Net.FABNetv4Ext",
			req:     permission.Request{Networks: []permission.Network{{Type: "FABNetv4Ext"}}},
			tags:    tagsExcept(permission.TagNetFABNetv4Ext),
			wantTag: permission.TagNetFABNetv4Ext,
		},
		{
			name:    "empty tags blocks a non-trivial request",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Cores: 8}}},
			tags:    []string{},
			wantTag: permission.TagVMNoLimitCPU,
		},
		{
			name:    "full tags allows the request",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Cores: 8, Disk: 100, Components: []permission.Component{{Type: "GPU", Model: "RTX6000"}}}}},
			tags:    allTags,
			wantTag: "",
		},
		{
			name:    "minimal request needs no tags",
			req:     permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Cores: 2, RAM: 8, Disk: 10}}},
			tags:    []string{},
			wantTag: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			validatePermissionTags(tc.req, fakeTokenSource{tags: tc.tags}, &diags)

			if tc.wantTag == "" {
				if diags.HasError() {
					t.Fatalf("expected no diagnostics, got: %v", diags)
				}
				return
			}
			if !diags.HasError() {
				t.Fatalf("expected a permission error for missing %q, got none", tc.wantTag)
			}
			found := false
			for _, d := range diags.Errors() {
				if d.Summary() == "Missing FABRIC project tag" && strings.Contains(d.Detail(), tc.wantTag) {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected a 'Missing FABRIC project tag' error naming %q, got: %v", tc.wantTag, diags)
			}
		})
	}
}

// TestValidatePermissionTags_NoProjectClaims confirms a token with no project claims is
// treated as having no tags, so any tag-requiring request is blocked (the provider does
// not fall through to the orchestrator).
func TestValidatePermissionTags_NoProjectClaims(t *testing.T) {
	t.Parallel()
	var diags diag.Diagnostics
	req := permission.Request{Nodes: []permission.Node{{Name: "vm1", Site: "RENC", Cores: 8}}}
	validatePermissionTags(req, fakeTokenSource{tags: nil}, &diags)
	if !diags.HasError() {
		t.Fatal("expected permission error when the token carries no project tags")
	}
}
