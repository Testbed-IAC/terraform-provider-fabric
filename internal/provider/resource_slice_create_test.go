package provider

import (
	"context"
	"testing"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_ResourceSlice_Create_ClientContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
	}{
		{name: "fake client records create"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{}
			_, err := client.CreateSlice(context.Background(), "slice", "<graphml/>", []string{"ssh-rsa x"}, fabricclient.CreateOpts{LifetimeHours: 24})
			if err != nil {
				t.Fatalf("CreateSlice returned error: %v", err)
			}
			if len(client.Calls) != 1 || client.Calls[0] != "CreateSlice:slice" {
				t.Fatalf("calls = %#v", client.Calls)
			}
		})
	}
}
