package provider

import (
	"context"
	"testing"
)

import "github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"

func TestFabric_DataSource_Resources_ClientLookup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		level        int32
		forceRefresh bool
	}{
		{name: "default level", level: 1},
		{name: "force refresh", level: 2, forceRefresh: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{}
			if _, err := client.GetResources(context.Background(), tc.level, tc.forceRefresh); err != nil {
				t.Fatalf("GetResources returned error: %v", err)
			}
			if len(client.Calls) != 1 || client.Calls[0] != "GetResources" {
				t.Fatalf("calls = %#v", client.Calls)
			}
		})
	}
}

func TestAccFabric_DataSource_Resources(t *testing.T) {
	t.Parallel()
	if !acceptanceEnabled() {
		t.Skip("acceptance tests require TF_ACC=1, FABRIC_TOKEN, and FABRIC_PROJECT_ID")
	}
}
