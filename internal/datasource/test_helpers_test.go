package datasource

import (
	"context"
	"testing"

	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// containsCall reports whether the fake client's recorded Calls slice contains
// want (e.g. "GetSlice:slice-1"), used to assert which client method a data source
// Read invoked.
func containsCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

// configFromModel encodes a typed data source model into a tfsdk.Config for the
// given schema by round-tripping it through a tfsdk.State. Every data source unit
// test builds its Config this way, so the encoding lives here once instead of being
// copied per data source.
func configFromModel[T any](t *testing.T, ctx context.Context, schema dschema.Schema, model T) tfsdk.Config {
	t.Helper()
	state := &tfsdk.State{Schema: schema}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("encoding data source model: %v", diags)
	}
	return tfsdk.Config{Schema: schema, Raw: state.Raw}
}
