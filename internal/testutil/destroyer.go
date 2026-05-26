package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

func CheckSliceDestroyed(ctx context.Context, t *testing.T, c fabricclient.FabricClient, sliceID string) {
	t.Helper()
	slice, err := c.GetSlice(ctx, sliceID)
	if errors.Is(err, fabricclient.ErrNotFound) {
		return
	}
	if err != nil {
		t.Fatalf("checking destroyed slice: %v", err)
	}
	if slice != nil && slice.State != "Dead" {
		t.Fatalf("slice %s still exists in state %s", sliceID, slice.State)
	}
}
