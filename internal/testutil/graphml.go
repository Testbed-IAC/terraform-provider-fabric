package testutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
)

// SliceGraph wraps the orchestrator-returned ASM topology for a slice. It is
// obtained by fetching GET /slices/{id}?graph_format=GRAPHML through the existing
// testmode client (Client) and parsing the returned model with the fabric-go-fim
// topology package — the same fetch+parse path the provider's Read uses, so no
// new HTTP or XML code is introduced here.
//
// Property key names are owned by fabric-go-fim/pkg/sliver/constants.go and the
// typed accessors below read them through sliver.NodeSliver. A live testmode bare
// VM ASM node carries: Type=VM, Name, Site, NodeID, GraphID, ImageRef
// ("default_rocky_9,qcow2"), Capacities {core,ram,disk}, MgmtIp, and
// ReservationInfo {reservation_id, reservation_state:"Active"}.
type SliceGraph struct {
	SliceID string
	topo    *topology.Topology
}

// FetchSliceGraph fetches and parses the ASM for sliceID from the testmode
// orchestrator. Token threading matches the rest of testutil: Client mints a
// fresh full-permission token internally.
func FetchSliceGraph(sliceID string) (*SliceGraph, error) {
	client, err := Client()
	if err != nil {
		return nil, err
	}
	slice, err := client.GetSlice(context.Background(), sliceID)
	if err != nil {
		return nil, fmt.Errorf("fetching slice %s graph: %w", sliceID, err)
	}
	if slice.Model == "" {
		return nil, fmt.Errorf("slice %s returned an empty GraphML model", sliceID)
	}
	topo, err := topology.Load(strings.NewReader(slice.Model))
	if err != nil {
		return nil, fmt.Errorf("parsing slice %s GraphML: %w", sliceID, err)
	}
	return &SliceGraph{SliceID: sliceID, topo: topo}, nil
}

// Node returns the typed NodeSliver for the named NetworkNode.
func (g *SliceGraph) Node(name string) (*sliver.NodeSliver, error) {
	node, ok := g.topo.Node(name)
	if !ok {
		return nil, fmt.Errorf("node %q not found in slice %s topology", name, g.SliceID)
	}
	return node.Sliver()
}

// VMNodes returns the typed NodeSliver for every NetworkNode whose Type is VM.
// Facility and Switch NetworkNodes are excluded.
func (g *SliceGraph) VMNodes() ([]*sliver.NodeSliver, error) {
	var out []*sliver.NodeSliver
	for _, node := range g.topo.Nodes() {
		s, err := node.Sliver()
		if err != nil {
			return nil, fmt.Errorf("reading node %q sliver in slice %s: %w", node.Name(), g.SliceID, err)
		}
		if s.Type == sliver.NodeTypeVM {
			out = append(out, s)
		}
	}
	return out, nil
}

// NodeComponents returns the typed ComponentSliver list directly owned by the
// named node.
func (g *SliceGraph) NodeComponents(nodeName string) ([]*sliver.ComponentSliver, error) {
	node, ok := g.topo.Node(nodeName)
	if !ok {
		return nil, fmt.Errorf("node %q not found in slice %s topology", nodeName, g.SliceID)
	}
	var out []*sliver.ComponentSliver
	for _, component := range node.Components() {
		s, err := component.Sliver()
		if err != nil {
			return nil, fmt.Errorf("reading component %q sliver: %w", component.Name(), err)
		}
		out = append(out, s)
	}
	return out, nil
}

// NetworkServiceNames returns the Name of every NetworkService in the topology
// (including component-internal services such as OVS).
func (g *SliceGraph) NetworkServiceNames() []string {
	var out []string
	for _, service := range g.topo.NetworkServices() {
		out = append(out, service.Name())
	}
	return out
}

// HasComponent reports whether the named node owns a component of the given type
// and (when model is non-empty) model.
func (g *SliceGraph) HasComponent(nodeName string, componentType sliver.ComponentType, model string) (bool, error) {
	components, err := g.NodeComponents(nodeName)
	if err != nil {
		return false, err
	}
	for _, component := range components {
		if component.Type != componentType {
			continue
		}
		if model == "" || component.Model == model {
			return true, nil
		}
	}
	return false, nil
}

// sliceIDFromState reads the slice_id attribute of resourceName from Terraform
// state, falling back to id.
func sliceIDFromState(state *terraform.State, resourceName string) (string, error) {
	rs, ok := state.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", resourceName)
	}
	sliceID := rs.Primary.Attributes["slice_id"]
	if sliceID == "" {
		sliceID = rs.Primary.Attributes["id"]
	}
	if sliceID == "" {
		return "", fmt.Errorf("resource %s has no slice_id in state", resourceName)
	}
	return sliceID, nil
}

// CheckSliceGraph fetches the ASM for the slice backing resourceName and runs fn
// against the parsed topology. Use it to assert topology-specific structure that
// the flat state attributes do not expose.
func CheckSliceGraph(resourceName string, fn func(*SliceGraph) error) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		sliceID, err := sliceIDFromState(state, resourceName)
		if err != nil {
			return err
		}
		graph, err := FetchSliceGraph(sliceID)
		if err != nil {
			return err
		}
		return fn(graph)
	}
}

// StandardSliceGraphChecks is the baseline ASM assertion every apply test runs.
// For each VM NetworkNode it proves the orchestrator persisted the provider-built
// identity (Name/Site/Type/NodeID/GraphID), defaulted the image and capacities,
// and that the slice actually allocated: a management IP from MockAM and an
// Active reservation. Every value asserted here is present on a live testmode
// StableOK bare VM (verified against the running stack), so this never produces a
// false failure for a slice that reached StableOK.
func StandardSliceGraphChecks(resourceName string) resource.TestCheckFunc {
	return CheckSliceGraph(resourceName, func(graph *SliceGraph) error {
		vms, err := graph.VMNodes()
		if err != nil {
			return err
		}
		if len(vms) == 0 {
			return fmt.Errorf("slice %s ASM contains no VM NetworkNode", graph.SliceID)
		}
		for _, vm := range vms {
			where := fmt.Sprintf("slice %s node %q", graph.SliceID, vm.Name)
			if vm.Name == "" {
				return fmt.Errorf("%s: empty Name", where)
			}
			if vm.Type != sliver.NodeTypeVM {
				return fmt.Errorf("%s: Type = %q, want VM", where, vm.Type)
			}
			if vm.Site == "" {
				return fmt.Errorf("%s: empty Site", where)
			}
			if vm.NodeID == "" {
				return fmt.Errorf("%s: empty NodeID", where)
			}
			if vm.GraphID == "" {
				return fmt.Errorf("%s: empty GraphID", where)
			}
			if vm.ImageRef == "" {
				return fmt.Errorf("%s: empty ImageRef", where)
			}
			if vm.Capacities == nil || vm.Capacities.Core <= 0 {
				return fmt.Errorf("%s: missing or zero Capacities.core (%v)", where, vm.Capacities)
			}
			if vm.MgmtIP == "" {
				return fmt.Errorf("%s: empty MgmtIp (MockAM did not provision)", where)
			}
			if vm.ReservationInfo == nil || vm.ReservationInfo.ReservationID == "" {
				return fmt.Errorf("%s: missing ReservationInfo.reservation_id (not allocated)", where)
			}
			if vm.ReservationInfo.ReservationState != "Active" {
				return fmt.Errorf("%s: reservation_state = %q, want Active", where, vm.ReservationInfo.ReservationState)
			}
		}
		return nil
	})
}

// CheckReservationMatchesSliverID cross-checks, for every VM node, the ASM's
// ReservationInfo.reservation_id against the nodes.<name>.sliver_id recorded in
// Terraform state. The provider sets sliver_id from ReservationInfo.ReservationID
// in setNodeOutputs, so the two must be identical for an allocated slice.
func CheckReservationMatchesSliverID(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		sliceID, err := sliceIDFromState(state, resourceName)
		if err != nil {
			return err
		}
		graph, err := FetchSliceGraph(sliceID)
		if err != nil {
			return err
		}
		vms, err := graph.VMNodes()
		if err != nil {
			return err
		}
		if len(vms) == 0 {
			return fmt.Errorf("slice %s ASM contains no VM NetworkNode", sliceID)
		}
		for _, vm := range vms {
			if vm.ReservationInfo == nil || vm.ReservationInfo.ReservationID == "" {
				return fmt.Errorf("slice %s node %q has no ReservationInfo.reservation_id in ASM", sliceID, vm.Name)
			}
			stateKey := fmt.Sprintf("nodes.%s.sliver_id", vm.Name)
			got := rs.Primary.Attributes[stateKey]
			if got == "" {
				return fmt.Errorf("state attribute %q is empty for resource %s", stateKey, resourceName)
			}
			if got != vm.ReservationInfo.ReservationID {
				return fmt.Errorf("node %q: state %s = %q, ASM reservation_id = %q", vm.Name, stateKey, got, vm.ReservationInfo.ReservationID)
			}
		}
		return nil
	}
}
