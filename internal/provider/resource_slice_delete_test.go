package provider

import (
	"context"
	"testing"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient/fake"
)

func TestFabric_ResourceSlice_Delete_ClientContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
	}{
		{name: "delete ok"},
		{name: "not found ok", err: fabricclient.ErrNotFound},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fake.Client{DeleteFn: func(context.Context, string) error { return tc.err }}
			err := client.DeleteSlice(context.Background(), "slice-1")
			if err != tc.err {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
		})
	}
}
