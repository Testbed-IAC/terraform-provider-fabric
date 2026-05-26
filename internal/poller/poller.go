package poller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

func WaitForSlice(ctx context.Context, c fabricclient.FabricClient, sliceID string, successStates, failureStates []string, timeout, interval time.Duration) (*fabricclient.Slice, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	success := stateSet(successStates)
	failure := stateSet(failureStates)

	for {
		slice, err := c.GetSlice(ctx, sliceID)
		if errors.Is(err, fabricclient.ErrNotFound) && success["Dead"] {
			return &fabricclient.Slice{SliceID: sliceID, State: "Dead"}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("polling slice %s: %w", sliceID, err)
		}
		if slice != nil {
			if success[slice.State] {
				return slice, nil
			}
			if failure[slice.State] {
				return slice, fmt.Errorf("polling slice %s reached %s: %s", sliceID, slice.State, slice.Notice)
			}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("polling slice %s: %w", sliceID, ctx.Err())
		case <-deadline.C:
			return nil, fmt.Errorf("polling slice %s: timeout after %s", sliceID, timeout)
		case <-ticker.C:
		}
	}
}

func stateSet(states []string) map[string]bool {
	out := make(map[string]bool, len(states))
	for _, state := range states {
		out[state] = true
	}
	return out
}
