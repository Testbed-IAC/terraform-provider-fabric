package poller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

const (
	// POASuccessState is the terminal success state returned by FABRIC POA.
	POASuccessState = "Success"
	// POAFailedState is the terminal failure state returned by FABRIC POA.
	POAFailedState = "Failed"
)

// WaitForSlice polls a FABRIC slice until it reaches a configured terminal state.
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

// WaitForPOA polls a FABRIC POA until it reaches Success or Failed.
func WaitForPOA(ctx context.Context, c fabricclient.FabricClient, poaID string, timeout, interval time.Duration) (*fabricclient.POA, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		poa, err := c.GetPOA(ctx, poaID)
		if err != nil {
			return nil, fmt.Errorf("polling poa %s: %w", poaID, err)
		}
		if poa != nil {
			switch poa.State {
			case POASuccessState:
				return poa, nil
			case POAFailedState:
				return poa, fmt.Errorf("polling poa %s reached %s: %s", poaID, poa.State, poa.Error)
			}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("polling poa %s: %w", poaID, ctx.Err())
		case <-deadline.C:
			return nil, fmt.Errorf("polling poa %s: timeout after %s", poaID, timeout)
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
