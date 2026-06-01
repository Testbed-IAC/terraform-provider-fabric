package provider

import (
	"context"
	"fmt"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/permission"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topologybuilder"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/userdata"
)

func buildTopology(ctx context.Context, model SliceResourceModel) (*topology.Topology, string, error) {
	spec, err := specFromModel(ctx, model)
	if err != nil {
		return nil, "", err
	}
	return topologybuilder.Build(spec)
}

func validateCatalog(ctx context.Context, model SliceResourceModel) error {
	spec, err := specFromModel(ctx, model)
	if err != nil {
		return err
	}
	return topologybuilder.ValidateCatalog(spec)
}

func specFromModel(ctx context.Context, model SliceResourceModel) (topologybuilder.SliceSpec, error) {
	spec := topologybuilder.SliceSpec{
		Name:          model.Name.ValueString(),
		LifetimeHours: int64Value(model.LifetimeHours),
	}
	for _, node := range model.Nodes {
		nodeSpec, err := nodeSpecFromModel(ctx, node)
		if err != nil {
			return topologybuilder.SliceSpec{}, err
		}
		spec.Nodes = append(spec.Nodes, nodeSpec)
	}
	for _, facility := range model.Facilities {
		facilitySpec, err := facilitySpecFromModel(facility)
		if err != nil {
			return topologybuilder.SliceSpec{}, err
		}
		spec.Facilities = append(spec.Facilities, facilitySpec)
	}
	for _, sw := range model.Switches {
		portLabels, err := sw.PortLabels.toFIM()
		if err != nil {
			return topologybuilder.SliceSpec{}, fmt.Errorf("building port labels for switch %s: %w", sw.Name.ValueString(), err)
		}
		spec.Switches = append(spec.Switches, topologybuilder.SwitchSpec{
			Name:       sw.Name.ValueString(),
			Site:       sw.Site.ValueString(),
			NPorts:     int64Value(sw.NPorts),
			PortLabels: portLabels,
		})
	}
	for _, network := range model.Networks {
		networkSpec, err := networkSpecFromModel(network)
		if err != nil {
			return topologybuilder.SliceSpec{}, err
		}
		spec.Networks = append(spec.Networks, networkSpec)
	}
	return spec, nil
}

func nodeSpecFromModel(ctx context.Context, node NodeModel) (topologybuilder.NodeSpec, error) {
	labels, err := node.Labels.toFIM()
	if err != nil {
		return topologybuilder.NodeSpec{}, fmt.Errorf("building labels for node %s: %w", node.Name.ValueString(), err)
	}
	postBootExecute, err := stringSliceValue(ctx, node.PostBootExecute)
	if err != nil {
		return topologybuilder.NodeSpec{}, fmt.Errorf("reading post_boot_execute for node %s: %w", node.Name.ValueString(), err)
	}
	postUpdate, err := stringSliceValue(ctx, node.PostUpdate)
	if err != nil {
		return topologybuilder.NodeSpec{}, fmt.Errorf("reading post_update for node %s: %w", node.Name.ValueString(), err)
	}
	out := topologybuilder.NodeSpec{
		Name:            node.Name.ValueString(),
		Site:            node.Site.ValueString(),
		Host:            stringValue(node.Host),
		InstanceType:    stringValue(node.InstanceType),
		ImageRef:        stringValue(node.ImageRef),
		ImageType:       stringValue(node.ImageType),
		Cores:           int64Value(node.Cores),
		RAM:             int64Value(node.RAM),
		Disk:            int64Value(node.Disk),
		BootScript:      stringValue(node.BootScript),
		PostBootExecute: postBootExecute,
		PostUpdate:      postUpdate,
		Labels:          labels,
	}
	for _, component := range node.Components {
		componentLabels, err := component.Labels.toFIM()
		if err != nil {
			return topologybuilder.NodeSpec{}, fmt.Errorf("building labels for component %s: %w", component.Name.ValueString(), err)
		}
		out.Components = append(out.Components, topologybuilder.ComponentSpec{
			Name:       component.Name.ValueString(),
			Type:       sliver.ComponentType(stringValue(component.Type)),
			Model:      stringValue(component.Model),
			FABlibName: stringValue(component.FABlibName),
			Labels:     componentLabels,
		})
	}
	for _, storage := range node.Storage {
		out.Storage = append(out.Storage, topologybuilder.StorageSpec{
			Name:      storage.Name.ValueString(),
			Model:     stringValue(storage.Model),
			AutoMount: boolValue(storage.AutoMount),
		})
	}
	for _, route := range node.Routes {
		out.Routes = append(out.Routes, userdata.Route{
			Subnet:  stringValue(route.Subnet),
			NextHop: stringValue(route.NextHop),
		})
	}
	for _, upload := range node.PostBootUploads {
		out.PostBootUploads = append(out.PostBootUploads, topologybuilder.PostBootUploadSpec{
			LocalPath:  stringValue(upload.LocalPath),
			RemotePath: stringValue(upload.RemotePath),
		})
	}
	return out, nil
}

func facilitySpecFromModel(facility FacilityPortModel) (topologybuilder.FacilitySpec, error) {
	labels, err := facility.Labels.toFIM()
	if err != nil {
		return topologybuilder.FacilitySpec{}, fmt.Errorf("building labels for facility %s: %w", facility.Name.ValueString(), err)
	}
	out := topologybuilder.FacilitySpec{
		Name:      facility.Name.ValueString(),
		Site:      facility.Site.ValueString(),
		VLAN:      stringValue(facility.VLAN),
		Bandwidth: int64Value(facility.Bandwidth),
		MTU:       int64Value(facility.MTU),
		Labels:    labels,
	}
	for _, iface := range facility.Interfaces {
		ifaceLabels, err := iface.Labels.toFIM()
		if err != nil {
			return topologybuilder.FacilitySpec{}, fmt.Errorf("building labels for facility %s interface %s: %w", facility.Name.ValueString(), iface.Name.ValueString(), err)
		}
		out.Interfaces = append(out.Interfaces, topologybuilder.FacilityInterfaceSpec{
			Name:   iface.Name.ValueString(),
			VLAN:   stringValue(iface.VLAN),
			Labels: ifaceLabels,
		})
	}
	return out, nil
}

func networkSpecFromModel(network NetworkModel) (topologybuilder.NetworkSpec, error) {
	labels, err := network.Labels.toFIM()
	if err != nil {
		return topologybuilder.NetworkSpec{}, fmt.Errorf("building labels for network %s: %w", network.Name.ValueString(), err)
	}
	gateway, err := gatewaySpecFromModel(network.Gateway)
	if err != nil {
		return topologybuilder.NetworkSpec{}, fmt.Errorf("building gateway for network %s: %w", network.Name.ValueString(), err)
	}
	out := topologybuilder.NetworkSpec{
		Name:            network.Name.ValueString(),
		Type:            sliver.ServiceType(network.Type.ValueString()),
		Bandwidth:       int64Value(network.Bandwidth),
		Site:            stringValue(network.Site),
		Technology:      stringValue(network.Technology),
		Subnet:          stringValue(network.Subnet),
		Gateway:         gateway,
		MirrorFrom:      network.MirrorFrom.ValueString(),
		MirrorDirection: sliver.MirrorDirection(stringValue(network.MirrorDirection)),
		Labels:          labels,
	}
	for _, iface := range network.Interfaces {
		ifaceSpec, err := interfaceSpecFromModel(iface)
		if err != nil {
			return topologybuilder.NetworkSpec{}, fmt.Errorf("building interface for network %s: %w", network.Name.ValueString(), err)
		}
		out.Interfaces = append(out.Interfaces, ifaceSpec)
	}
	return out, nil
}

func interfaceSpecFromModel(iface InterfaceModel) (topologybuilder.InterfaceRef, error) {
	labels, err := iface.Labels.toFIM()
	if err != nil {
		return topologybuilder.InterfaceRef{}, fmt.Errorf("building labels: %w", err)
	}
	out := topologybuilder.InterfaceRef{
		Node:      iface.Node.ValueString(),
		Component: stringValue(iface.Component),
		Facility:  stringValue(iface.Facility),
		Port:      int64Value(iface.Port),
		Name:      stringValue(iface.Name),
		Labels:    labels,
	}
	for _, sub := range iface.SubInterfaces {
		subLabels, err := sub.Labels.toFIM()
		if err != nil {
			return topologybuilder.InterfaceRef{}, fmt.Errorf("building labels for sub-interface %s: %w", sub.Name.ValueString(), err)
		}
		out.SubInterfaces = append(out.SubInterfaces, topologybuilder.SubInterfaceSpec{
			Name:      sub.Name.ValueString(),
			VLAN:      stringValue(sub.VLAN),
			Bandwidth: int64Value(sub.Bandwidth),
			Labels:    subLabels,
		})
	}
	return out, nil
}

func gatewaySpecFromModel(model *GatewayModel) (*sliver.Gateway, error) {
	if model == nil {
		return nil, nil
	}
	gateway := sliver.Gateway{
		IPv4:       stringValue(model.IPv4),
		IPv4Subnet: stringValue(model.IPv4Subnet),
		IPv6:       stringValue(model.IPv6),
		IPv6Subnet: stringValue(model.IPv6Subnet),
		MAC:        stringValue(model.MAC),
	}
	if gateway.Empty() {
		return nil, nil
	}
	if err := gateway.Validate(); err != nil {
		return nil, err
	}
	return &gateway, nil
}

func permissionRequest(model SliceResourceModel) permission.Request {
	spec := topologybuilder.SliceSpec{
		LifetimeHours: int64Value(model.LifetimeHours),
	}
	for _, node := range model.Nodes {
		nodeSpec := topologybuilder.NodeSpec{
			Name:  node.Name.ValueString(),
			Site:  node.Site.ValueString(),
			Cores: int64Value(node.Cores),
			RAM:   int64Value(node.RAM),
			Disk:  int64Value(node.Disk),
		}
		for _, component := range node.Components {
			nodeSpec.Components = append(nodeSpec.Components, topologybuilder.ComponentSpec{
				Type:  sliver.ComponentType(stringValue(component.Type)),
				Model: stringValue(component.Model),
			})
		}
		spec.Nodes = append(spec.Nodes, nodeSpec)
	}
	for _, network := range model.Networks {
		spec.Networks = append(spec.Networks, topologybuilder.NetworkSpec{
			Type:      sliver.ServiceType(network.Type.ValueString()),
			Bandwidth: int64Value(network.Bandwidth),
		})
	}
	return topologybuilder.PermissionRequest(spec)
}
