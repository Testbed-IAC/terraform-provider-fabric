package provider

import (
	"context"
	"fmt"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/permission"
)

func buildTopology(ctx context.Context, model SliceResourceModel) (*topology.Topology, string, error) {
	_ = ctx
	topo := topology.NewWithID(topology.DeriveGraphID(model.Name.ValueString()))
	nodes := map[string]*topology.Node{}

	for _, node := range model.Nodes {
		labels, err := nodeLabels(node)
		if err != nil {
			return nil, "", fmt.Errorf("building labels for node %s: %w", node.Name.ValueString(), err)
		}
		opts := topology.NodeOpts{
			Name:      node.Name.ValueString(),
			Site:      node.Site.ValueString(),
			Type:      sliver.NodeTypeVM,
			ImageRef:  defaultString(stringValue(node.ImageRef), "default_rocky_9"),
			ImageType: defaultString(stringValue(node.ImageType), "qcow2"),
			Labels:    labels,
		}
		// instance_type and explicit capacities are alternative sizing
		// mechanisms. When an instance type is set the orchestrator derives the
		// capacities from the flavor, so we must not also inject the default
		// 2/8/10 capacities: doing so would make the orchestrator allocate the
		// tiny default instead of the requested flavor. Explicit cores/ram/disk
		// still override individual dimensions when both are set.
		if instanceType := stringValue(node.InstanceType); instanceType != "" {
			opts.CapacityHints = &sliver.CapacityHints{InstanceType: instanceType}
			if caps, ok := explicitCapacities(node); ok {
				opts.Capacities = &caps
			}
		} else {
			caps := capacitiesFromNode(node)
			opts.Capacities = &caps
		}
		built, err := topo.AddNode(opts)
		if err != nil {
			return nil, "", fmt.Errorf("adding node %s: %w", node.Name.ValueString(), err)
		}
		nodes[node.Name.ValueString()] = built
		for _, component := range node.Components {
			componentLabels, err := component.Labels.toFIM()
			if err != nil {
				return nil, "", fmt.Errorf("building labels for component %s: %w", component.Name.ValueString(), err)
			}
			componentOpts := topology.ComponentOpts{
				Name:       component.Name.ValueString(),
				Type:       sliver.ComponentType(stringValue(component.Type)),
				Model:      stringValue(component.Model),
				FABlibName: stringValue(component.FABlibName),
				Labels:     componentLabels,
			}
			if _, err := built.AddComponent(componentOpts); err != nil {
				return nil, "", fmt.Errorf("adding component %s: %w", component.Name.ValueString(), err)
			}
		}
	}

	for _, network := range model.Networks {
		networkLabels, err := network.Labels.toFIM()
		if err != nil {
			return nil, "", fmt.Errorf("building labels for network %s: %w", network.Name.ValueString(), err)
		}
		if network.Type.ValueString() == "PortMirror" {
			firstIface := firstInterface(network)
			toInterface, err := resolveInterface(nodes, firstIface)
			if err != nil {
				return nil, "", fmt.Errorf("resolving mirror destination: %w", err)
			}
			ifaceLabels, err := firstIface.Labels.toFIM()
			if err != nil {
				return nil, "", fmt.Errorf("building labels for port mirror %s interface: %w", network.Name.ValueString(), err)
			}
			if err := toInterface.SetLabels(ifaceLabels); err != nil {
				return nil, "", fmt.Errorf("setting labels for port mirror %s interface: %w", network.Name.ValueString(), err)
			}
			_, err = topo.AddPortMirrorService(topology.PortMirrorOpts{
				Name:              network.Name.ValueString(),
				FromInterfaceName: network.MirrorFrom.ValueString(),
				ToInterface:       toInterface,
				Direction:         sliver.MirrorDirection(stringValue(network.MirrorDirection)),
				Labels:            networkLabels,
			})
			if err != nil {
				return nil, "", fmt.Errorf("adding port mirror %s: %w", network.Name.ValueString(), err)
			}
			continue
		}
		ifaces := make([]*topology.Interface, 0, len(network.Interfaces))
		for _, ifaceModel := range network.Interfaces {
			iface, err := resolveInterface(nodes, ifaceModel)
			if err != nil {
				return nil, "", fmt.Errorf("resolving interface for network %s: %w", network.Name.ValueString(), err)
			}
			ifaceLabels, err := ifaceModel.Labels.toFIM()
			if err != nil {
				return nil, "", fmt.Errorf("building labels for network %s interface: %w", network.Name.ValueString(), err)
			}
			if err := iface.SetLabels(ifaceLabels); err != nil {
				return nil, "", fmt.Errorf("setting labels for network %s interface: %w", network.Name.ValueString(), err)
			}
			ifaces = append(ifaces, iface)
		}
		opts := topology.NetworkServiceOpts{
			Name:       network.Name.ValueString(),
			Type:       sliver.ServiceType(network.Type.ValueString()),
			Interfaces: ifaces,
			Labels:     networkLabels,
		}
		if bw := int64Value(network.Bandwidth); bw > 0 {
			opts.Capacities = &sliver.Capacities{BW: int(bw)}
		}
		if _, err := topo.AddNetworkService(opts); err != nil {
			return nil, "", fmt.Errorf("adding network %s: %w", network.Name.ValueString(), err)
		}
	}

	graphML, err := topo.SerializeString()
	if err != nil {
		return nil, "", fmt.Errorf("serializing topology: %w", err)
	}
	return topo, graphML, nil
}

func validateCatalog(model SliceResourceModel) error {
	instances, err := catalog.Instances()
	if err != nil {
		return fmt.Errorf("loading instance catalog: %w", err)
	}
	components, err := catalog.Components()
	if err != nil {
		return fmt.Errorf("loading component catalog: %w", err)
	}
	for _, node := range model.Nodes {
		if !node.InstanceType.IsNull() && !node.InstanceType.IsUnknown() && node.InstanceType.ValueString() != "" {
			if _, ok := instances.Lookup(node.InstanceType.ValueString()); !ok {
				return fmt.Errorf("looking up instance type %s: %w", node.InstanceType.ValueString(), catalog.ErrNotFound)
			}
		}
		for _, component := range node.Components {
			componentType := sliver.ComponentType(stringValue(component.Type))
			componentModel := stringValue(component.Model)
			if fablibName := stringValue(component.FABlibName); fablibName != "" {
				resolvedType, resolvedModel, ok := catalog.ResolveFABlibModel(fablibName)
				if !ok {
					return fmt.Errorf("resolving FABlib model %s: %w", fablibName, catalog.ErrNotFound)
				}
				componentType = resolvedType
				componentModel = resolvedModel
			}
			if _, ok := components.Lookup(componentType, componentModel); !ok {
				return fmt.Errorf("looking up component %s/%s: %w", componentType, componentModel, catalog.ErrNotFound)
			}
		}
	}
	return nil
}

func nodeLabels(node NodeModel) (*sliver.Labels, error) {
	labels, err := node.Labels.toFIM()
	if err != nil {
		return nil, err
	}
	host := stringValue(node.Host)
	if host == "" {
		return labels, nil
	}
	if labels == nil {
		return &sliver.Labels{InstanceParent: host}, nil
	}
	if labels.InstanceParent != "" && labels.InstanceParent != host {
		return nil, fmt.Errorf("node host %q conflicts with labels.instance_parent %q", host, labels.InstanceParent)
	}
	labels.InstanceParent = host
	return labels, nil
}

// explicitCapacities returns the capacity dimensions the user set on the node,
// without applying any defaults. ok is false when no dimension was set.
func explicitCapacities(node NodeModel) (sliver.Capacities, bool) {
	caps := sliver.Capacities{
		Core: int(int64Value(node.Cores)),
		RAM:  int(int64Value(node.RAM)),
		Disk: int(int64Value(node.Disk)),
	}
	return caps, !caps.Empty()
}

func capacitiesFromNode(node NodeModel) sliver.Capacities {
	core := int64Value(node.Cores)
	ram := int64Value(node.RAM)
	disk := int64Value(node.Disk)
	if core == 0 {
		core = 2
	}
	if ram == 0 {
		ram = 8
	}
	if disk == 0 {
		disk = 10
	}
	return sliver.Capacities{Core: int(core), RAM: int(ram), Disk: int(disk)}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func resolveInterface(nodes map[string]*topology.Node, model InterfaceModel) (*topology.Interface, error) {
	nodeName := model.Node.ValueString()
	node, ok := nodes[nodeName]
	if !ok {
		return nil, fmt.Errorf("node %q: %w", nodeName, topology.ErrNotFound)
	}
	componentName := stringValue(model.Component)
	port := int(int64Value(model.Port))
	if componentName != "" {
		for _, component := range node.Components() {
			if component.Name() != componentName && component.Name() != node.Name()+"-"+componentName {
				continue
			}
			interfaces := component.Interfaces()
			if port < 0 || port >= len(interfaces) {
				return nil, fmt.Errorf("component %q port %d: %w", componentName, port, topology.ErrNotFound)
			}
			return interfaces[port], nil
		}
		return nil, fmt.Errorf("component %q: %w", componentName, topology.ErrNotFound)
	}
	if name := stringValue(model.Name); name != "" {
		for _, iface := range node.InterfaceList() {
			if iface.Name() == name {
				return iface, nil
			}
		}
		return nil, fmt.Errorf("interface %q: %w", name, topology.ErrNotFound)
	}
	interfaces := node.InterfaceList()
	if port < 0 || port >= len(interfaces) {
		return nil, fmt.Errorf("node %q port %d: %w", nodeName, port, topology.ErrNotFound)
	}
	return interfaces[port], nil
}

func firstInterface(network NetworkModel) InterfaceModel {
	if len(network.Interfaces) == 0 {
		return InterfaceModel{}
	}
	return network.Interfaces[0]
}

func permissionRequest(model SliceResourceModel) permission.Request {
	req := permission.Request{
		LifetimeHours: int64Value(model.LifetimeHours),
	}
	for _, node := range model.Nodes {
		n := permission.Node{
			Name:  node.Name.ValueString(),
			Site:  node.Site.ValueString(),
			Cores: int64Value(node.Cores),
			RAM:   int64Value(node.RAM),
			Disk:  int64Value(node.Disk),
		}
		for _, component := range node.Components {
			n.Components = append(n.Components, permission.Component{
				Type:  stringValue(component.Type),
				Model: stringValue(component.Model),
			})
		}
		req.Nodes = append(req.Nodes, n)
	}
	for _, network := range model.Networks {
		req.Networks = append(req.Networks, permission.Network{
			Type:      network.Type.ValueString(),
			Bandwidth: int64Value(network.Bandwidth),
		})
	}
	return req
}
