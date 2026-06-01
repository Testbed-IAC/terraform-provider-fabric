package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// TestFabric_TopologyBuilder_AdvancedVMParseable verifies that a VM with
// explicit capacities (cores=4/ram=16/disk=50) produces GraphML that is
// well-formed XML and parses cleanly via topology.Load. It does NOT diff
// against the bare_vm fixture because the capacities differ.
func TestFabric_TopologyBuilder_AdvancedVMParseable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		model SliceResourceModel
	}{
		{name: "advanced vm 4/16/50", model: advancedVMModel()},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, built, err := buildTopology(context.Background(), tc.model)
			if err != nil {
				t.Fatalf("buildTopology returned error: %v", err)
			}
			if built == "" {
				t.Fatalf("buildTopology returned empty GraphML")
			}
			// Must be parseable round-trip.
			loaded, err := topology.Load(strings.NewReader(built))
			if err != nil {
				t.Fatalf("topology.Load returned error: %v\ngraphml:\n%s", err, built)
			}
			node, ok := loaded.Node("vm1")
			if !ok {
				t.Fatalf("loaded topology missing node vm1\ngraphml:\n%s", built)
			}
			sliver, err := node.Sliver()
			if err != nil {
				t.Fatalf("node.Sliver returned error: %v", err)
			}
			if sliver.Capacities == nil {
				t.Fatalf("loaded node has nil Capacities\ngraphml:\n%s", built)
			}
			if sliver.Capacities.Core != 4 || sliver.Capacities.RAM != 16 || sliver.Capacities.Disk != 50 {
				t.Fatalf("loaded capacities = %+v, want core=4 ram=16 disk=50", sliver.Capacities)
			}
			if sliver.Site != "RENC" {
				t.Fatalf("loaded site = %q, want RENC", sliver.Site)
			}
			// Must not contain the duplicate xmlns that previously broke the
			// orchestrator's Neo4j importer.
			if strings.Count(built, `xmlns="http://graphml.graphdrawing.org/xmlns"`) != 1 {
				t.Fatalf("GraphML must contain exactly one default xmlns attribute, got:\n%s", built)
			}
			// Must not emit a redundant <data key="labels"> child — labels is
			// already carried by the labels XML attribute on <node>.
			if strings.Contains(built, `<data key="labels">`) {
				t.Fatalf("GraphML must not emit <data key=\"labels\">, got:\n%s", built)
			}
		})
	}
}

// TestFabric_TopologyBuilder_InstanceTypeOmitsDefaultCapacities guards against a
// regression where a node configured with instance_type also received the
// default 2/8/10 capacities. The orchestrator derives capacities from the
// flavor, so emitting default capacities alongside an instance type would make
// it allocate the tiny default instead of the requested flavor.
func TestFabric_TopologyBuilder_InstanceTypeOmitsDefaultCapacities(t *testing.T) {
	t.Parallel()
	const instanceType = "fabric.c8.m32.d100"
	model := SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{{
			Name:         types.StringValue("vm1"),
			Site:         types.StringValue("RENC"),
			InstanceType: types.StringValue(instanceType),
		}},
	}
	_, built, err := buildTopology(context.Background(), model)
	if err != nil {
		t.Fatalf("buildTopology returned error: %v", err)
	}
	loaded, err := topology.Load(strings.NewReader(built))
	if err != nil {
		t.Fatalf("topology.Load returned error: %v", err)
	}
	node, ok := loaded.Node("vm1")
	if !ok {
		t.Fatalf("loaded topology missing node vm1")
	}
	sliver, err := node.Sliver()
	if err != nil {
		t.Fatalf("node.Sliver returned error: %v", err)
	}
	if sliver.CapacityHints == nil || sliver.CapacityHints.InstanceType != instanceType {
		t.Fatalf("capacity hints = %+v, want instance_type %q", sliver.CapacityHints, instanceType)
	}
	if sliver.Capacities != nil {
		t.Fatalf("capacities = %+v, want nil when instance_type drives sizing", sliver.Capacities)
	}
}

// TestFabric_TopologyBuilder_InstanceTypeWithExplicitCapacities verifies that
// explicit cores/ram/disk still override individual dimensions when an
// instance_type is also set.
func TestFabric_TopologyBuilder_InstanceTypeWithExplicitCapacities(t *testing.T) {
	t.Parallel()
	model := SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{{
			Name:         types.StringValue("vm1"),
			Site:         types.StringValue("RENC"),
			InstanceType: types.StringValue("fabric.c8.m32.d100"),
			Disk:         types.Int64Value(200),
		}},
	}
	_, built, err := buildTopology(context.Background(), model)
	if err != nil {
		t.Fatalf("buildTopology returned error: %v", err)
	}
	loaded, err := topology.Load(strings.NewReader(built))
	if err != nil {
		t.Fatalf("topology.Load returned error: %v", err)
	}
	node, _ := loaded.Node("vm1")
	sliver, err := node.Sliver()
	if err != nil {
		t.Fatalf("node.Sliver returned error: %v", err)
	}
	if sliver.Capacities == nil || sliver.Capacities.Disk != 200 {
		t.Fatalf("capacities = %+v, want explicit disk=200", sliver.Capacities)
	}
	// Dimensions the user did not set must stay zero, not default to 2/8.
	if sliver.Capacities.Core != 0 || sliver.Capacities.RAM != 0 {
		t.Fatalf("capacities = %+v, want core=0 ram=0 (flavor-derived)", sliver.Capacities)
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

// advancedVMModel is a VM with explicit capacities (cores=4, ram=16, disk=50).
func advancedVMModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("tf-advanced-vm"),
		Nodes: []NodeModel{{
			Name:      types.StringValue("vm1"),
			Site:      types.StringValue("RENC"),
			ImageRef:  types.StringValue("default_rocky_9"),
			ImageType: types.StringValue("qcow2"),
			Cores:     types.Int64Value(4),
			RAM:       types.Int64Value(16),
			Disk:      types.Int64Value(50),
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

func crossSiteL2BridgeModel() SliceResourceModel {
	model := l2BridgeModel()
	model.Nodes[1].Site = types.StringValue("UKY")
	return model
}
