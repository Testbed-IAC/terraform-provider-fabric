package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_ResourceSlice_Read_ClientStates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		state string
		err   error
	}{
		{name: "not found", err: fabricclient.ErrNotFound},
		{name: "dead", state: "Dead"},
		{name: "stable error", state: "StableError"},
		{name: "modify error", state: "ModifyError"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return &fabricclient.Slice{SliceID: "slice-1", State: tc.state}, nil
			}}
			got, err := client.GetSlice(context.Background(), "slice-1")
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if tc.err == nil && got.State != tc.state {
				t.Fatalf("state = %q, want %q", got.State, tc.state)
			}
		})
	}
}
