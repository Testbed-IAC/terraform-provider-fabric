package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestFabric_Provider_Schema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
	}{
		{name: "schema contains required configuration attributes"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &FabricProvider{version: "test"}
			var resp provider.SchemaResponse
			p.Schema(context.Background(), provider.SchemaRequest{}, &resp)
			for _, attr := range []string{"token", "orchestrator_url", "project_id"} {
				if _, ok := resp.Schema.Attributes[attr]; !ok {
					t.Fatalf("missing provider attribute %s", attr)
				}
			}
		})
	}
}
