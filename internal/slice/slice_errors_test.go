// Tests for the slice resource's client-error translation layer — addClientError,
// pdpDiagnostic, projectName, and the pdpTagPattern regex. These functions turn raw
// fabricclient sentinel errors and orchestrator PDP/Policy-Violation HTTP 500 bodies
// into actionable, tag-aware diagnostics. They are reachable in production only
// through real orchestrator HTTP failures (so the acceptance suite cannot provoke
// them deterministically), yet they carry the densest branching in the package;
// these table tests exercise every branch against crafted errors and a fake token
// source. No network or stack is involved.
package slice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

// claimsTokenSource is a minimal auth.TokenSource backed by fixed claims, so the
// error-translation tests can drive projectName/pdpDiagnostic without minting a JWT.
type claimsTokenSource struct {
	claims *auth.Claims
}

func (f claimsTokenSource) IDToken(context.Context) (string, error) { return "", nil }
func (f claimsTokenSource) ProjectID() string                       { return "" }
func (f claimsTokenSource) Claims() *auth.Claims                    { return f.claims }

func claimsWith(name, uuid string, tags ...string) *auth.Claims {
	return &auth.Claims{Projects: []auth.Project{{Name: name, UUID: uuid, Tags: tags}}}
}

func TestSliceAddClientError(t *testing.T) {
	t.Parallel()
	r := &SliceResource{tokenSource: claimsTokenSource{claims: claimsWith("MyProject", "proj-uuid", "VM.NoLimit")}}
	cases := []struct {
		name          string
		err           error
		wantSummary   string
		wantDetailHas string
	}{
		{"unauthorized", fmt.Errorf("wrap: %w", fabricclient.ErrUnauthorized), "FABRIC Authentication Failed", "401 Unauthorized"},
		{"forbidden", fmt.Errorf("wrap: %w", fabricclient.ErrForbidden), "FABRIC Authorization Failed", "MyProject"},
		{"bad request", fmt.Errorf("wrap: %w", fabricclient.ErrBadRequest), "Invalid Slice Configuration", "topology"},
		{"server error", fmt.Errorf("wrap: %w", fabricclient.ErrServerError), "FABRIC Orchestrator Error", "500 Internal Server Error"},
		{"unknown falls through to default summary", errors.New("some unexpected failure"), "Create FABRIC slice failed", "some unexpected failure"},
		{"pdp policy violation wins over sentinel mapping", errors.New("HTTP 500: PDP Failure: Your project is lacking VM.NoLimitCPU or VM.NoLimit tag to provision"), "FABRIC Policy Violation", "VM.NoLimitCPU or VM.NoLimit"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			r.addClientError(&diags, "Create FABRIC slice failed", tc.err)
			if len(diags.Errors()) != 1 {
				t.Fatalf("errors = %d, want 1: %v", len(diags.Errors()), diags)
			}
			got := diags.Errors()[0]
			if got.Summary() != tc.wantSummary {
				t.Fatalf("summary = %q, want %q", got.Summary(), tc.wantSummary)
			}
			if !strings.Contains(got.Detail(), tc.wantDetailHas) {
				t.Fatalf("detail = %q, want it to contain %q", got.Detail(), tc.wantDetailHas)
			}
		})
	}
}

func TestSlicePDPDiagnostic(t *testing.T) {
	t.Parallel()
	t.Run("non-pdp error is not recognized", func(t *testing.T) {
		t.Parallel()
		r := &SliceResource{tokenSource: claimsTokenSource{claims: claimsWith("P", "u")}}
		if _, _, ok := r.pdpDiagnostic(errors.New("connection refused")); ok {
			t.Fatal("ok = true, want false for a non-PDP error")
		}
	})
	t.Run("missing tag is extracted and known tags are listed sorted", func(t *testing.T) {
		t.Parallel()
		r := &SliceResource{tokenSource: claimsTokenSource{claims: claimsWith("P", "u", "VM.NoLimitRAM", "Component.GPU")}}
		_, detail, ok := r.pdpDiagnostic(errors.New("Policy Violation: project is lacking Component.GPU tag"))
		if !ok {
			t.Fatal("ok = false, want true for a Policy Violation error")
		}
		if !strings.Contains(detail, "missing the Component.GPU tag") {
			t.Fatalf("detail = %q, want it to name the missing tag", detail)
		}
		// Tags are sorted, so Component.GPU precedes VM.NoLimitRAM.
		if !strings.Contains(detail, "Component.GPU, VM.NoLimitRAM") {
			t.Fatalf("detail = %q, want sorted known tags", detail)
		}
	})
	t.Run("pdp without a parseable tag falls back to the generic variant", func(t *testing.T) {
		t.Parallel()
		r := &SliceResource{tokenSource: claimsTokenSource{claims: claimsWith("P", "u")}}
		_, detail, ok := r.pdpDiagnostic(errors.New("PDP Failure: request denied"))
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if !strings.Contains(detail, "(none discovered from token)") {
			t.Fatalf("detail = %q, want the no-tags placeholder", detail)
		}
	})
}

func TestSliceProjectName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ts   auth.TokenSource
		want string
	}{
		{"name preferred", claimsTokenSource{claims: claimsWith("Named", "uuid-1")}, "Named"},
		{"uuid when name empty", claimsTokenSource{claims: claimsWith("", "uuid-1")}, "uuid-1"},
		{"unknown when both empty", claimsTokenSource{claims: claimsWith("", "")}, "unknown project"},
		{"unknown when token source is nil", nil, "unknown project"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &SliceResource{tokenSource: tc.ts}
			if got := r.projectName(); got != tc.want {
				t.Fatalf("projectName() = %q, want %q", got, tc.want)
			}
		})
	}
}
