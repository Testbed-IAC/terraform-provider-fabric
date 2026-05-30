package fabricclient

import (
	"context"
	"testing"
)

type countingTokenSource struct {
	count  int
	token  string
	claims *FabricClaims
}

func (c *countingTokenSource) IDToken(context.Context) (string, error) {
	c.count++
	return c.token, nil
}

func (c *countingTokenSource) ProjectID() string {
	return c.claims.ProjectID()
}

func (c *countingTokenSource) Claims() *FabricClaims {
	return c.claims
}

func TestFabric_FabricClient_AdapterAuthCtx_UsesTokenSourceEachCall(t *testing.T) {
	t.Parallel()
	ts := &countingTokenSource{token: "token-1", claims: &FabricClaims{}}
	adapter := New("https://orchestrator.example", ts)
	for i := 0; i < 2; i++ {
		if _, err := adapter.authCtx(context.Background()); err != nil {
			t.Fatalf("authCtx returned error: %v", err)
		}
	}
	if ts.count != 2 {
		t.Fatalf("token source calls = %d, want 2", ts.count)
	}
}
