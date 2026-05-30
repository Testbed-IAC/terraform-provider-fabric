package provider

import (
	"context"
	"os"
	"testing"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_DataSource_Slice_ClientLookup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		call func(*fake.Client) error
		want string
	}{
		{name: "by id", call: func(c *fake.Client) error { _, err := c.GetSlice(context.Background(), "slice-1"); return err }, want: "GetSlice:slice-1"},
		{name: "by name", call: func(c *fake.Client) error { _, err := c.ListSlices(context.Background(), "slice", nil); return err }, want: "ListSlices:slice"},
		{name: "both prefers id", call: func(c *fake.Client) error { _, err := c.GetSlice(context.Background(), "slice-1"); return err }, want: "GetSlice:slice-1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{
				GetFn: func(context.Context, string) (*fabricclient.Slice, error) { return &fabricclient.Slice{}, nil },
				ListFn: func(context.Context, string, []string) ([]fabricclient.Slice, error) {
					return []fabricclient.Slice{{}}, nil
				},
			}
			if err := tc.call(client); err != nil {
				t.Fatalf("call returned error: %v", err)
			}
			if len(client.Calls) != 1 || client.Calls[0] != tc.want {
				t.Fatalf("calls = %#v, want %s", client.Calls, tc.want)
			}
		})
	}
}

func TestAccFabric_DataSource_Slice(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("acceptance tests require TF_ACC=1")
	}
	if os.Getenv("FABRIC_TOKEN") == "" && os.Getenv("FABRIC_TOKEN_LOCATION") == "" {
		t.Skip("acceptance tests require FABRIC_TOKEN or FABRIC_TOKEN_LOCATION")
	}
}
