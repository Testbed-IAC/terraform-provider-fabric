package provider

import (
	"context"
	"testing"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_ResourceSlice_Update_ClientContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		call func(*fake.Client) error
		want string
	}{
		{name: "modify", call: func(c *fake.Client) error {
			_, err := c.ModifySlice(context.Background(), "slice-1", "graph")
			return err
		}, want: "ModifySlice:slice-1"},
		{name: "accept", call: func(c *fake.Client) error { _, err := c.AcceptModify(context.Background(), "slice-1"); return err }, want: "AcceptModify:slice-1"},
		{name: "renew", call: func(c *fake.Client) error {
			return c.RenewSlice(context.Background(), "slice-1", "2026-05-27T00:00:00Z")
		}, want: "RenewSlice:slice-1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{AcceptFn: func(context.Context, string) (*fabricclient.Slice, error) { return &fabricclient.Slice{}, nil }}
			if err := tc.call(client); err != nil {
				t.Fatalf("call returned error: %v", err)
			}
			if len(client.Calls) != 1 || client.Calls[0] != tc.want {
				t.Fatalf("calls = %#v, want %s", client.Calls, tc.want)
			}
		})
	}
}
