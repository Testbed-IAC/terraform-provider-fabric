// Tests for the Terraform-value extraction helpers in values.go. They convert
// framework types.List / []types.String into plain Go slices while dropping
// null/unknown elements — behavior that is load-bearing where the slice resource
// and POA resource forward optional list inputs (post_boot_execute, node_set, bdf)
// to fabric-go-fim. These are pure functions, so the tests are table-driven with no
// context or stack.
package tfutil

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStringSliceValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := []struct {
		name string
		list types.List
		want []string
	}{
		{"null list returns nil", types.ListNull(types.StringType), nil},
		{"unknown list returns nil", types.ListUnknown(types.StringType), nil},
		{
			name: "known values are returned in order",
			list: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("b")}),
			want: []string{"a", "b"},
		},
		{
			name: "null elements are dropped",
			list: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a"), types.StringNull(), types.StringValue("c")}),
			want: []string{"a", "c"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := StringSliceValue(ctx, tc.list)
			if err != nil {
				t.Fatalf("StringSliceValue error: %v", err)
			}
			if !equalStrings(got, tc.want) {
				t.Fatalf("StringSliceValue = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestStringValues(t *testing.T) {
	t.Parallel()
	in := []types.String{types.StringValue("x"), types.StringNull(), types.StringUnknown(), types.StringValue("y")}
	got := StringValues(in)
	want := []string{"x", "y"}
	if !equalStrings(got, want) {
		t.Fatalf("StringValues = %#v, want %#v (null/unknown must be dropped)", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
