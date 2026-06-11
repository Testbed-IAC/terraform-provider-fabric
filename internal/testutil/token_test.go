package testutil_test

import (
	"slices"
	"testing"
	"time"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestToken_RoundTrips verifies the minter produces a token the provider's own parser
// accepts, with the expected project, tags, and a future expiry. Run this before any
// acceptance test — it is the contract the whole suite depends on.
func TestToken_FullTokenRoundTrips(t *testing.T) {
	t.Parallel()
	claims, err := auth.ParseJWT(testutil.FullToken())
	if err != nil {
		t.Fatalf("ParseJWT(FullToken()) error: %v", err)
	}
	if got := claims.ProjectID(); got != testutil.TestProjectUUID {
		t.Fatalf("ProjectID = %q, want %q", got, testutil.TestProjectUUID)
	}
	for _, tag := range testutil.AllTags {
		if !claims.HasTag(tag) {
			t.Errorf("FullToken missing tag %q", tag)
		}
	}
	if exp := claims.ExpiresAt(); !exp.After(time.Now()) {
		t.Fatalf("FullToken exp = %v, want future", exp)
	}
}

func TestToken_TokenWithTags(t *testing.T) {
	t.Parallel()
	claims, err := auth.ParseJWT(testutil.TokenWithTags("VM.NoLimitCPU"))
	if err != nil {
		t.Fatalf("ParseJWT error: %v", err)
	}
	if !claims.HasTag("VM.NoLimitCPU") {
		t.Error("expected VM.NoLimitCPU present")
	}
	if claims.HasTag("Component.GPU") {
		t.Error("did not expect Component.GPU")
	}
}

func TestToken_EmptyTags(t *testing.T) {
	t.Parallel()
	claims, err := auth.ParseJWT(testutil.TokenWithTags())
	if err != nil {
		t.Fatalf("ParseJWT error: %v", err)
	}
	if len(claims.Project().Tags) != 0 {
		t.Fatalf("tags = %v, want empty", claims.Project().Tags)
	}
	if claims.ProjectID() != testutil.TestProjectUUID {
		t.Fatalf("ProjectID = %q, want %q", claims.ProjectID(), testutil.TestProjectUUID)
	}
}

func TestToken_NoProjects(t *testing.T) {
	t.Parallel()
	claims, err := auth.ParseJWT(testutil.TokenWithProjects(nil))
	if err != nil {
		t.Fatalf("ParseJWT error: %v", err)
	}
	if claims.ProjectID() != "" {
		t.Fatalf("ProjectID = %q, want empty for no-project token", claims.ProjectID())
	}
}

func TestToken_ExpiredRejectedByProvider(t *testing.T) {
	t.Parallel()
	// The provider's StaticToken enforces exp before any HTTP call (C3): minting an
	// expired token and building a static source must surface the token as unusable.
	ts, err := auth.NewStaticToken(testutil.ExpiredToken())
	if err != nil {
		t.Fatalf("NewStaticToken error: %v", err)
	}
	if _, err := ts.IDToken(t.Context()); err == nil {
		t.Fatal("expected IDToken to reject an expired token, got nil error")
	}
}

func TestTagsExcept(t *testing.T) {
	t.Parallel()
	got := testutil.TagsExcept("Component.GPU")
	if slices.Contains(got, "Component.GPU") {
		t.Error("TagsExcept should have removed Component.GPU")
	}
	if !slices.Contains(got, "VM.NoLimitCPU") {
		t.Error("TagsExcept dropped an unrelated tag")
	}
}
