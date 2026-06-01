package slice

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topologybuilder"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/userdata"
)

// TestFabric_TopologyBuilder_FeatureFixtures diffs provider-built topologies for
// the new feature surfaces (gateway/FABNetv4, explicit L2PTP, facility ports,
// switches) against the FIM Python golden fixtures. A non-empty semantic diff
// fails the test, so these assert real structural equivalence — same node
// names, classes, types, capacities, labels, and sites.
func TestFabric_TopologyBuilder_FeatureFixtures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fixture string
		model   SliceResourceModel
	}{
		{name: "fabnetv4 gateway", fixture: "fabnetv4.graphml", model: fabnetv4Model()},
		{name: "l2ptp explicit", fixture: "l2ptp.graphml", model: l2ptpModel()},
		{name: "facility port", fixture: "facility_port.graphml", model: facilityPortModel()},
		{name: "switch node", fixture: "switch_node.graphml", model: switchNodeModel()},
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

// TestFabric_TopologyBuilder_GatewayRoundTrip verifies that a gateway block on a
// FABNetv4 service reaches the serialized GraphML and survives a reload with the
// exact IPv4 address and subnet.
func TestFabric_TopologyBuilder_GatewayRoundTrip(t *testing.T) {
	t.Parallel()
	_, built, err := buildTopology(context.Background(), fabnetv4Model())
	if err != nil {
		t.Fatalf("buildTopology returned error: %v", err)
	}
	loaded, err := topology.Load(strings.NewReader(built))
	if err != nil {
		t.Fatalf("topology.Load returned error: %v", err)
	}
	svc, ok := loaded.NetworkService("v4net")
	if !ok {
		t.Fatalf("loaded topology missing service v4net")
	}
	sl, err := svc.Sliver()
	if err != nil {
		t.Fatalf("svc.Sliver returned error: %v", err)
	}
	if sl.Type != sliver.ServiceTypeFABNetv4 {
		t.Fatalf("service type = %q, want FABNetv4", sl.Type)
	}
	if sl.Gateway == nil {
		t.Fatalf("service gateway = nil, want IPv4 gateway")
	}
	if sl.Gateway.IPv4 != "10.0.0.1" || sl.Gateway.IPv4Subnet != "10.0.0.0/24" {
		t.Fatalf("gateway = %+v, want IPv4 10.0.0.1 subnet 10.0.0.0/24", sl.Gateway)
	}
}

// TestFabric_TopologyBuilder_UserDataRoundTrip verifies that boot_script, routes,
// post_boot_execute, post_boot_upload, post_update, and storage auto_mount are
// assembled into the node user-data envelope and survive serialization.
func TestFabric_TopologyBuilder_UserDataRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	model := SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{{
			Name:            types.StringValue("vm1"),
			Site:            types.StringValue("RENC"),
			BootScript:      types.StringValue("#!/bin/sh\necho boot"),
			PostBootExecute: stringList(t, ctx, "systemctl restart nginx", "uname -a"),
			PostUpdate:      stringList(t, ctx, "dnf upgrade -y"),
			Routes: []RouteModel{{
				Subnet:  types.StringValue("10.0.0.0/24"),
				NextHop: types.StringValue("10.0.1.1"),
			}},
			PostBootUploads: []PostBootUploadModel{{
				LocalPath:  types.StringValue("/local/file"),
				RemotePath: types.StringValue("/remote/file"),
			}},
			Storage: []StorageModel{{
				Name:      types.StringValue("vol1"),
				AutoMount: types.BoolValue(true),
			}},
		}},
	}
	_, built, err := buildTopology(ctx, model)
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
	sl, err := node.Sliver()
	if err != nil {
		t.Fatalf("node.Sliver returned error: %v", err)
	}
	if sl.BootScript != "#!/bin/sh\necho boot" {
		t.Fatalf("boot script = %q, want the configured script", sl.BootScript)
	}
	if len(sl.UserData) == 0 {
		t.Fatalf("user-data is empty, want assembled envelope")
	}
	data, err := userdata.Decode(sl.UserData)
	if err != nil {
		t.Fatalf("userdata.Decode returned error: %v", err)
	}
	if len(data.Routes) != 1 || data.Routes[0].Subnet != "10.0.0.0/24" || data.Routes[0].NextHop != "10.0.1.1" {
		t.Fatalf("routes = %+v, want one 10.0.0.0/24 -> 10.0.1.1", data.Routes)
	}
	if len(data.PostUpdate) != 1 || data.PostUpdate[0] != "dnf upgrade -y" {
		t.Fatalf("post_update = %+v, want [dnf upgrade -y]", data.PostUpdate)
	}
	if !data.Storage {
		t.Fatalf("storage flag = false, want true from auto_mount")
	}
	// post_boot_execute (2 commands) + post_boot_upload (1) = 3 ordered tasks.
	if len(data.PostBootTasks) != 3 {
		t.Fatalf("post_boot_tasks = %+v, want 3 tasks", data.PostBootTasks)
	}
	if data.PostBootTasks[0].Type != userdata.TaskExecute || data.PostBootTasks[0].Args[0] != "systemctl restart nginx" {
		t.Fatalf("first task = %+v, want execute systemctl restart nginx", data.PostBootTasks[0])
	}
	if data.PostBootTasks[2].Type != userdata.TaskUploadFile || data.PostBootTasks[2].Args[1] != "/remote/file" {
		t.Fatalf("third task = %+v, want upload_file to /remote/file", data.PostBootTasks[2])
	}
	// The storage block also adds a Storage component to the node.
	foundStorage := false
	for _, comp := range node.Components() {
		csl, err := comp.Sliver()
		if err != nil {
			t.Fatalf("component.Sliver returned error: %v", err)
		}
		if csl.Type == sliver.ComponentTypeStorage {
			foundStorage = true
		}
	}
	if !foundStorage {
		t.Fatalf("expected a Storage component on vm1")
	}
}

// TestFabric_TopologyBuilder_EmptyUserDataOmitted verifies that a node with no
// user-data inputs does not carry a user-data envelope (no empty "{}" blob).
func TestFabric_TopologyBuilder_EmptyUserDataOmitted(t *testing.T) {
	t.Parallel()
	_, built, err := buildTopology(context.Background(), bareVMModel())
	if err != nil {
		t.Fatalf("buildTopology returned error: %v", err)
	}
	loaded, err := topology.Load(strings.NewReader(built))
	if err != nil {
		t.Fatalf("topology.Load returned error: %v", err)
	}
	node, _ := loaded.Node("vm1")
	sl, err := node.Sliver()
	if err != nil {
		t.Fatalf("node.Sliver returned error: %v", err)
	}
	if len(sl.UserData) != 0 {
		t.Fatalf("user-data = %q, want empty for a bare VM", sl.UserData)
	}
}

// TestFabric_TopologyBuilder_SubInterface verifies that a sub_interface block on
// a network interface produces a VLAN-tagged child interface under the parent
// DedicatedPort.
func TestFabric_TopologyBuilder_SubInterface(t *testing.T) {
	t.Parallel()
	model := SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("snic1"), Type: types.StringValue("SmartNIC"), Model: types.StringValue("ConnectX-6")}}},
			{Name: types.StringValue("vm2"), Site: types.StringValue("UKY"), Components: []ComponentModel{{Name: types.StringValue("snic1"), Type: types.StringValue("SmartNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{{
			Name: types.StringValue("ptp1"),
			Type: types.StringValue("L2PTP"),
			Interfaces: []InterfaceModel{
				{
					Node:      types.StringValue("vm1"),
					Component: types.StringValue("snic1"),
					Port:      types.Int64Value(0),
					SubInterfaces: []SubInterfaceModel{{
						Name: types.StringValue("sub100"),
						VLAN: types.StringValue("100"),
					}},
				},
				{Node: types.StringValue("vm2"), Component: types.StringValue("snic1"), Port: types.Int64Value(0)},
			},
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
	vm, ok := loaded.Node("vm1")
	if !ok {
		t.Fatalf("loaded topology missing node vm1")
	}
	found := false
	for _, comp := range vm.Components() {
		for _, port := range comp.InterfaceList() {
			for _, child := range port.ChildInterfaces() {
				found = true
				csl, err := child.Sliver()
				if err != nil {
					t.Fatalf("child.Sliver returned error: %v", err)
				}
				if csl.Labels == nil || csl.Labels.VLAN != "100" {
					t.Fatalf("sub-interface %q VLAN = %+v, want 100", child.Name(), csl.Labels)
				}
			}
		}
	}
	if !found {
		t.Fatalf("no sub-interface found under vm1 components")
	}
}

// TestFabric_TopologyBuilder_InferredType exercises the omitted-type path: the
// provider must select the same L2 service type FABlib would for the connected
// interfaces and sites.
func TestFabric_TopologyBuilder_InferredType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		model SliceResourceModel
		want  sliver.ServiceType
	}{
		{name: "same-site shared NICs -> L2Bridge", model: inferredModel("RENC", "RENC"), want: sliver.ServiceTypeL2Bridge},
		{name: "cross-site shared NICs -> L2STS", model: inferredModel("RENC", "UKY"), want: sliver.ServiceTypeL2STS},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, built, err := buildTopology(context.Background(), tc.model)
			if err != nil {
				t.Fatalf("buildTopology returned error: %v", err)
			}
			loaded, err := topology.Load(strings.NewReader(built))
			if err != nil {
				t.Fatalf("topology.Load returned error: %v", err)
			}
			svc, ok := loaded.NetworkService("lan1")
			if !ok {
				t.Fatalf("loaded topology missing service lan1")
			}
			sl, err := svc.Sliver()
			if err != nil {
				t.Fatalf("svc.Sliver returned error: %v", err)
			}
			if sl.Type != tc.want {
				t.Fatalf("inferred type = %q, want %q", sl.Type, tc.want)
			}
		})
	}
}

// TestResolveMirrorDirection verifies the lowercase aliases map to the canonical
// FIM mirror directions and that canonical values pass through unchanged.
func TestResolveMirrorDirection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want sliver.MirrorDirection
	}{
		{in: "", want: ""},
		{in: "both", want: sliver.MirrorBoth},
		{in: "Both", want: sliver.MirrorBoth},
		{in: "rx", want: sliver.MirrorRXOnly},
		{in: "RX_Only", want: sliver.MirrorRXOnly},
		{in: "tx", want: sliver.MirrorTXOnly},
		{in: "TX_Only", want: sliver.MirrorTXOnly},
	}
	for _, tc := range cases {
		if got := topologybuilder.NormalizeMirrorDirection(sliver.MirrorDirection(tc.in)); got != tc.want {
			t.Fatalf("resolveMirrorDirection(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFabric_TopologyBuilder_MirrorDirectionAlias drives the lowercase alias end
// to end: a port-mirror service built with mirror_direction="rx" must serialize
// the canonical RX_Only direction.
func TestFabric_TopologyBuilder_MirrorDirectionAlias(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sourceName := mirrorSourceInterfaceName(t, ctx)
	model := mirrorModel(sourceName, "rx")
	_, built, err := buildTopology(ctx, model)
	if err != nil {
		t.Fatalf("buildTopology returned error: %v", err)
	}
	loaded, err := topology.Load(strings.NewReader(built))
	if err != nil {
		t.Fatalf("topology.Load returned error: %v", err)
	}
	svc, ok := loaded.NetworkService("pm1")
	if !ok {
		t.Fatalf("loaded topology missing service pm1")
	}
	sl, err := svc.Sliver()
	if err != nil {
		t.Fatalf("svc.Sliver returned error: %v", err)
	}
	if sl.MirrorDirection != sliver.MirrorRXOnly {
		t.Fatalf("mirror direction = %q, want RX_Only", sl.MirrorDirection)
	}
}

// TestFabric_TopologyBuilder_FacilityL2STSGolden covers the original Python FIM
// pattern of connecting a facility port to a SmartNIC data port through L2STS.
// The provider already had separate facility and sub-interface coverage; this
// golden keeps their Terraform-to-FIM mapping working as one generated graph.
func TestFabric_TopologyBuilder_FacilityL2STSGolden(t *testing.T) {
	t.Parallel()
	model := facilityL2STSModel()
	_, built, err := buildTopology(context.Background(), model)
	if err != nil {
		t.Fatalf("buildTopology returned error: %v", err)
	}
	loaded, err := topology.Load(strings.NewReader(built))
	if err != nil {
		t.Fatalf("topology.Load returned error: %v", err)
	}
	assertFacilityL2STS(t, loaded)

	normalized := normalizeGraphML(built)
	goldenPath := filepath.Join("testdata", "facility_l2sts_smartnic_port.graphml")
	if *update {
		if err := os.WriteFile(goldenPath, []byte(built), 0o644); err != nil {
			t.Fatalf("updating golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if normalizeGraphML(string(want)) != normalized {
		t.Fatalf("GraphML golden mismatch; run go test ./internal/slice -run TestFabric_TopologyBuilder_FacilityL2STSGolden -update to regenerate")
	}
}

func stringList(t *testing.T, ctx context.Context, values ...string) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(ctx, types.StringType, values)
	if diags.HasError() {
		t.Fatalf("building string list: %v", diags)
	}
	return list
}

func fabnetv4Model() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
			{Name: types.StringValue("vm2"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{{
			Name: types.StringValue("v4net"),
			Type: types.StringValue("FABNetv4"),
			Interfaces: []InterfaceModel{
				{Node: types.StringValue("vm1"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
				{Node: types.StringValue("vm2"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
			},
			Gateway: &GatewayModel{
				IPv4:       types.StringValue("10.0.0.1"),
				IPv4Subnet: types.StringValue("10.0.0.0/24"),
			},
		}},
	}
}

func l2ptpModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("snic1"), Type: types.StringValue("SmartNIC"), Model: types.StringValue("ConnectX-6")}}},
			{Name: types.StringValue("vm2"), Site: types.StringValue("UKY"), Components: []ComponentModel{{Name: types.StringValue("snic1"), Type: types.StringValue("SmartNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{{
			Name: types.StringValue("ptp1"),
			Type: types.StringValue("L2PTP"),
			Interfaces: []InterfaceModel{
				{Node: types.StringValue("vm1"), Component: types.StringValue("snic1"), Port: types.Int64Value(0)},
				{Node: types.StringValue("vm2"), Component: types.StringValue("snic1"), Port: types.Int64Value(0)},
			},
		}},
	}
}

func facilityPortModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Facilities: []FacilityPortModel{{
			Name:      types.StringValue("ESnet-DTN"),
			Site:      types.StringValue("RENC"),
			VLAN:      types.StringValue("100"),
			Bandwidth: types.Int64Value(10),
		}},
	}
}

func facilityL2STSModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("facility-l2sts"),
		Nodes: []NodeModel{{
			Name: types.StringValue("vm1"),
			Site: types.StringValue("UKY"),
			Components: []ComponentModel{{
				Name:  types.StringValue("snic1"),
				Type:  types.StringValue("SmartNIC"),
				Model: types.StringValue("ConnectX-6"),
			}},
		}},
		Facilities: []FacilityPortModel{{
			Name:      types.StringValue("RENCI-DTN"),
			Site:      types.StringValue("RENC"),
			VLAN:      types.StringValue("100"),
			Bandwidth: types.Int64Value(10),
		}},
		Networks: []NetworkModel{{
			Name: types.StringValue("s-fac"),
			Type: types.StringValue("L2STS"),
			Interfaces: []InterfaceModel{
				{Facility: types.StringValue("RENCI-DTN")},
				{Node: types.StringValue("vm1"), Component: types.StringValue("snic1"), Port: types.Int64Value(1)},
			},
		}},
	}
}

func assertFacilityL2STS(t *testing.T, loaded *topology.Topology) {
	t.Helper()
	service, ok := loaded.NetworkService("s-fac")
	if !ok {
		t.Fatalf("loaded topology missing service s-fac")
	}
	serviceSliver, err := service.Sliver()
	if err != nil {
		t.Fatalf("service.Sliver returned error: %v", err)
	}
	if serviceSliver.Type != sliver.ServiceTypeL2STS {
		t.Fatalf("service type = %q, want L2STS", serviceSliver.Type)
	}
	if len(service.Interfaces()) != 2 {
		t.Fatalf("service interfaces = %d, want 2", len(service.Interfaces()))
	}
	facility, ok := loaded.Node("RENCI-DTN")
	if !ok {
		t.Fatalf("loaded topology missing facility RENCI-DTN")
	}
	facilitySliver, err := facility.Sliver()
	if err != nil {
		t.Fatalf("facility.Sliver returned error: %v", err)
	}
	if facilitySliver.Type != sliver.NodeTypeFacility {
		t.Fatalf("facility node type = %q, want Facility", facilitySliver.Type)
	}
	iface := findInterface(t, facility, "RENCI-DTN-int")
	ifaceSliver, err := iface.Sliver()
	if err != nil {
		t.Fatalf("facility interface Sliver returned error: %v", err)
	}
	if ifaceSliver.Labels == nil || ifaceSliver.Labels.VLAN != "100" {
		t.Fatalf("facility interface labels = %+v, want VLAN 100", ifaceSliver.Labels)
	}
	if ifaceSliver.Capacities == nil || ifaceSliver.Capacities.BW != 10 {
		t.Fatalf("facility interface capacities = %+v, want bandwidth 10", ifaceSliver.Capacities)
	}
	vm, ok := loaded.Node("vm1")
	if !ok {
		t.Fatalf("loaded topology missing vm1")
	}
	iface = findInterface(t, vm, "snic1-p2")
	if iface == nil {
		t.Fatalf("missing vm1 SmartNIC port snic1-p2")
	}
}

func findInterface(t *testing.T, node *topology.Node, name string) *topology.Interface {
	t.Helper()
	var names []string
	for _, iface := range node.InterfaceList() {
		names = append(names, iface.Name())
		if iface.Name() == name {
			return iface
		}
	}
	t.Fatalf("node %s missing interface %s; available interfaces: %s", node.Name(), name, strings.Join(names, ", "))
	return nil
}

func switchNodeModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Switches: []SwitchModel{{
			Name:   types.StringValue("sw1"),
			Site:   types.StringValue("RENC"),
			NPorts: types.Int64Value(4),
		}},
	}
}

func inferredModel(siteA, siteB string) SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue(siteA), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
			{Name: types.StringValue("vm2"), Site: types.StringValue(siteB), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{{
			Name: types.StringValue("lan1"),
			Interfaces: []InterfaceModel{
				{Node: types.StringValue("vm1"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
				{Node: types.StringValue("vm2"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
			},
		}},
	}
}

// mirrorModel builds a topology with a source L2Bridge and a PortMirror service
// whose source interface is named sourceName, using the given direction alias.
func mirrorModel(sourceName, direction string) SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
			{Name: types.StringValue("vm2"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{
			{
				Name: types.StringValue("lan1"),
				Type: types.StringValue("L2Bridge"),
				Interfaces: []InterfaceModel{
					{Node: types.StringValue("vm1"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
				},
			},
			{
				Name:            types.StringValue("pm1"),
				Type:            types.StringValue("PortMirror"),
				MirrorFrom:      types.StringValue(sourceName),
				MirrorDirection: types.StringValue(direction),
				Interfaces: []InterfaceModel{
					{Node: types.StringValue("vm2"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
				},
			},
		},
	}
}

// mirrorSourceInterfaceName builds a source-only topology (no PortMirror) once
// to discover the auto-generated name of vm1's SharedNIC port, which the
// PortMirror service later references by name.
func mirrorSourceInterfaceName(t *testing.T, ctx context.Context) string {
	t.Helper()
	discovery := SliceResourceModel{
		Name: types.StringValue("slice"),
		Nodes: []NodeModel{
			{Name: types.StringValue("vm1"), Site: types.StringValue("RENC"), Components: []ComponentModel{{Name: types.StringValue("nic1"), Type: types.StringValue("SharedNIC"), Model: types.StringValue("ConnectX-6")}}},
		},
		Networks: []NetworkModel{{
			Name: types.StringValue("lan1"),
			Type: types.StringValue("L2Bridge"),
			Interfaces: []InterfaceModel{
				{Node: types.StringValue("vm1"), Component: types.StringValue("nic1"), Port: types.Int64Value(0)},
			},
		}},
	}
	topo, _, err := buildTopology(ctx, discovery)
	if err != nil {
		t.Fatalf("buildTopology (discovery) returned error: %v", err)
	}
	vm, ok := topo.Node("vm1")
	if !ok {
		t.Fatalf("discovery topology missing node vm1")
	}
	for _, comp := range vm.Components() {
		ifaces := comp.InterfaceList()
		if len(ifaces) > 0 {
			return ifaces[0].Name()
		}
	}
	t.Fatalf("vm1 has no component interfaces")
	return ""
}
