package provider

import (
	"context"
	"testing"
)

func TestFabric_SliceSchema_Attributes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		attr string
	}{
		{name: "id computed", attr: "id"},
		{name: "slice id computed", attr: "slice_id"},
		{name: "ssh key sensitive", attr: "ssh_key"},
		{name: "nodes computed map", attr: "nodes"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sliceResourceSchema(context.Background())
			if _, ok := s.Attributes[tc.attr]; !ok {
				t.Fatalf("missing slice schema attribute %s", tc.attr)
			}
		})
	}
}

func TestFabric_SliceSchema_Blocks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		block string
	}{
		{name: "node block", block: "node"},
		{name: "network block", block: "network"},
		{name: "timeouts block", block: "timeouts"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sliceResourceSchema(context.Background())
			if _, ok := s.Blocks[tc.block]; !ok {
				t.Fatalf("missing slice schema block %s", tc.block)
			}
		})
	}
}
