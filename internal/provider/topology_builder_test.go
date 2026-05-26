package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFabric_TopologyBuilder_RoundTripFixtures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fixture string
		model   SliceResourceModel
	}{
		{name: "bare vm", fixture: "bare_vm.graphml", model: bareVMModel()},
		{name: "shared nic", fixture: "vm_shared_nic.graphml", model: sharedNICModel()},
		{name: "l2bridge", fixture: "lan_l2bridge.graphml", model: l2BridgeModel()},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, built, err := buildTopology(context.Background(), tc.model)
			if err != nil {
				t.Fatalf("buildTopology returned error: %v", err)
			}
			fixture := readFixture(t, tc.fixture)
			diff, err := topology.DiffTopologyGraphML(fixture, []byte(built))
			if err != nil {
				t.Fatalf("DiffTopologyGraphML returned error: %v", err)
			}
			if !diff.Empty() {
				for _, d := range diff.Diagnostics() {
					t.Logf("%s: %s", d.Field(), d.Suggestion())
				}
				t.Fatalf("topology diff: %s", diff.Summary())
			}
		})
	}
}

func TestFabric_TopologyBuilder_Validation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		model SliceResourceModel
	}{
		{
			name: "invalid name",
			model: SliceResourceModel{
				Name:  types.StringValue("x"),
				Nodes: []NodeModel{{Name: types.StringValue("x"), Site: types.StringValue("RENC")}},
			},
		},
		{
			name: "site required",
			model: SliceResourceModel{
				Name:  types.StringValue("slice"),
				Nodes: []NodeModel{{Name: types.StringValue("vm1")}},
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := buildTopology(context.Background(), tc.model); err == nil {
				t.Fatalf("expected buildTopology error")
			}
		})
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "fabric-go-fim", "testdata", "fixtures", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return body
}

func bareVMModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{{
			Name: types.StringValue("vm1"),
			Site: types.StringValue("RENC"),
		}},
	}
}

func sharedNICModel() SliceResourceModel {
	m := bareVMModel()
	m.Nodes[0].Components = []ComponentModel{{
		Name:  types.StringValue("nic1"),
		Type:  types.StringValue("SharedNIC"),
		Model: types.StringValue("ConnectX-6"),
	},
	}
	return m
}

func l2BridgeModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
			{Name: types.StringValue("vm2"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{{
			Name: types.StringValue("lan1"),
			Type: types.StringValue("L2Bridge"),
			Interfaces: []InterfaceModel{
				{Node: types.StringValue("vm1"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
				{Node: types.StringValue("vm2"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
			},
		}},
	}
}
