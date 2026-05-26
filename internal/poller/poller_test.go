package poller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_Poller_States(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		states  []string
		success []string
		failure []string
		want    string
		wantErr string
	}{
		{name: "stable ok happy path", states: []string{"Nascent", "Configuring", "StableOK"}, success: []string{"StableOK"}, failure: []string{"StableError"}, want: "StableOK"},
		{name: "stable error fails", states: []string{"StableError"}, success: []string{"StableOK"}, failure: []string{"StableError"}, wantErr: "StableError"},
		{name: "allocated error fails", states: []string{"AllocatedError"}, success: []string{"StableOK"}, failure: []string{"AllocatedError"}, wantErr: "AllocatedError"},
		{name: "allocated ok continues polling", states: []string{"AllocatedOK", "StableOK"}, success: []string{"StableOK"}, failure: []string{"AllocatedError"}, want: "StableOK"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			i := 0
			client := &fake.Client{GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
				state := tc.states[i]
				if i < len(tc.states)-1 {
					i++
				}
				return &fabricclient.Slice{SliceID: "slice-1", State: state, Notice: "notice"}, nil
			}}
			got, err := WaitForSlice(context.Background(), client, "slice-1", tc.success, tc.failure, time.Second, time.Millisecond)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("WaitForSlice returned error: %v", err)
			}
			if got.State != tc.want {
				t.Fatalf("state = %q, want %q", got.State, tc.want)
			}
		})
	}
}

func TestFabric_Poller_ContextAndTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		ctx     func() context.Context
		timeout time.Duration
		wantErr string
	}{
		{name: "context cancel", ctx: canceledContext, timeout: time.Second, wantErr: "context canceled"},
		{name: "timeout", ctx: context.Background, timeout: time.Millisecond, wantErr: "timeout"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{GetFn: func(context.Context, string) (*fabricclient.Slice, error) {
				return &fabricclient.Slice{SliceID: "slice-1", State: "Configuring"}, nil
			}}
			_, err := WaitForSlice(tc.ctx(), client, "slice-1", []string{"StableOK"}, nil, tc.timeout, time.Millisecond)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
