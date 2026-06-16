package slice

import (
	"context"
	"fmt"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/permission"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topologybuilder"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/userdata"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// buildTopology converts the Terraform model into a FIM topology and its GraphML
// serialization.
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

// specFromModel converts the Terraform slice model into a topologybuilder.SliceSpec.
func specFromModel(ctx context.Context, model SliceResourceModel) (topologybuilder.SliceSpec, error) {
	spec := topologybuilder.SliceSpec{
		Name:          model.Name.ValueString(),
		LifetimeHours: tfutil.Int64Value(model.LifetimeHours),
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
		portLabels, err := labelsToFIM(sw.PortLabels)
		if err != nil {
			return topologybuilder.SliceSpec{}, fmt.Errorf("building port labels for switch %s: %w", sw.Name.ValueString(), err)
		}
		spec.Switches = append(spec.Switches, topologybuilder.SwitchSpec{
			Name:       sw.Name.ValueString(),
			Site:       sw.Site.ValueString(),
			NPorts:     tfutil.Int64Value(sw.NPorts),
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
	labels, err := labelsToFIM(node.Labels)
	if err != nil {
		return topologybuilder.NodeSpec{}, fmt.Errorf("building labels for node %s: %w", node.Name.ValueString(), err)
	}
	postBootExecute, err := tfutil.StringSliceValue(ctx, node.PostBootExecute)
	if err != nil {
		return topologybuilder.NodeSpec{}, fmt.Errorf("reading post_boot_execute for node %s: %w", node.Name.ValueString(), err)
	}
	postUpdate, err := tfutil.StringSliceValue(ctx, node.PostUpdate)
	if err != nil {
		return topologybuilder.NodeSpec{}, fmt.Errorf("reading post_update for node %s: %w", node.Name.ValueString(), err)
	}
	out := topologybuilder.NodeSpec{
		Name:            node.Name.ValueString(),
		Site:            node.Site.ValueString(),
		Host:            tfutil.StringValue(node.Host),
		InstanceType:    tfutil.StringValue(node.InstanceType),
		ImageRef:        tfutil.StringValue(node.ImageRef),
		ImageType:       tfutil.StringValue(node.ImageType),
		Cores:           tfutil.Int64Value(node.Cores),
		RAM:             tfutil.Int64Value(node.RAM),
		Disk:            tfutil.Int64Value(node.Disk),
		BootScript:      tfutil.StringValue(node.BootScript),
		PostBootExecute: postBootExecute,
		PostUpdate:      postUpdate,
		Labels:          labels,
	}
	for _, component := range node.Components {
		componentLabels, err := labelsToFIM(component.Labels)
		if err != nil {
			return topologybuilder.NodeSpec{}, fmt.Errorf("building labels for component %s: %w", component.Name.ValueString(), err)
		}
		out.Components = append(out.Components, topologybuilder.ComponentSpec{
			Name:       component.Name.ValueString(),
			Type:       sliver.ComponentType(tfutil.StringValue(component.Type)),
			Model:      tfutil.StringValue(component.Model),
			FABlibName: tfutil.StringValue(component.FABlibName),
			Labels:     componentLabels,
		})
	}
	for _, storage := range node.Storage {
		out.Storage = append(out.Storage, topologybuilder.StorageSpec{
			Name:      storage.Name.ValueString(),
			Model:     tfutil.StringValue(storage.Model),
			AutoMount: tfutil.BoolValue(storage.AutoMount),
		})
	}
	for _, route := range node.Routes {
		out.Routes = append(out.Routes, userdata.Route{
			Subnet:  tfutil.StringValue(route.Subnet),
			NextHop: tfutil.StringValue(route.NextHop),
		})
	}
	for _, upload := range node.PostBootUploads {
		out.PostBootUploads = append(out.PostBootUploads, topologybuilder.PostBootUploadSpec{
			LocalPath:  tfutil.StringValue(upload.LocalPath),
			RemotePath: tfutil.StringValue(upload.RemotePath),
		})
	}
	return out, nil
}

func facilitySpecFromModel(facility FacilityPortModel) (topologybuilder.FacilitySpec, error) {
	labels, err := labelsToFIM(facility.Labels)
	if err != nil {
		return topologybuilder.FacilitySpec{}, fmt.Errorf("building labels for facility %s: %w", facility.Name.ValueString(), err)
	}
	out := topologybuilder.FacilitySpec{
		Name:      facility.Name.ValueString(),
		Site:      facility.Site.ValueString(),
		VLAN:      tfutil.StringValue(facility.VLAN),
		Bandwidth: tfutil.Int64Value(facility.Bandwidth),
		MTU:       tfutil.Int64Value(facility.MTU),
		Labels:    labels,
	}
	for _, iface := range facility.Interfaces {
		ifaceLabels, err := labelsToFIM(iface.Labels)
		if err != nil {
			return topologybuilder.FacilitySpec{}, fmt.Errorf("building labels for facility %s interface %s: %w", facility.Name.ValueString(), iface.Name.ValueString(), err)
		}
		out.Interfaces = append(out.Interfaces, topologybuilder.FacilityInterfaceSpec{
			Name:   iface.Name.ValueString(),
			VLAN:   tfutil.StringValue(iface.VLAN),
			Labels: ifaceLabels,
		})
	}
	return out, nil
}

func networkSpecFromModel(network NetworkModel) (topologybuilder.NetworkSpec, error) {
	labels, err := labelsToFIM(network.Labels)
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
		Bandwidth:       tfutil.Int64Value(network.Bandwidth),
		Site:            tfutil.StringValue(network.Site),
		Technology:      tfutil.StringValue(network.Technology),
		Subnet:          tfutil.StringValue(network.Subnet),
		Gateway:         gateway,
		MirrorFrom:      network.MirrorFrom.ValueString(),
		MirrorDirection: sliver.MirrorDirection(tfutil.StringValue(network.MirrorDirection)),
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
	labels, err := labelsToFIM(iface.Labels)
	if err != nil {
		return topologybuilder.InterfaceRef{}, fmt.Errorf("building labels: %w", err)
	}
	out := topologybuilder.InterfaceRef{
		Node:      iface.Node.ValueString(),
		Component: tfutil.StringValue(iface.Component),
		Facility:  tfutil.StringValue(iface.Facility),
		Port:      tfutil.Int64Value(iface.Port),
		Name:      tfutil.StringValue(iface.Name),
		Labels:    labels,
	}
	for _, sub := range iface.SubInterfaces {
		subLabels, err := labelsToFIM(sub.Labels)
		if err != nil {
			return topologybuilder.InterfaceRef{}, fmt.Errorf("building labels for sub-interface %s: %w", sub.Name.ValueString(), err)
		}
		out.SubInterfaces = append(out.SubInterfaces, topologybuilder.SubInterfaceSpec{
			Name:      sub.Name.ValueString(),
			VLAN:      tfutil.StringValue(sub.VLAN),
			Bandwidth: tfutil.Int64Value(sub.Bandwidth),
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
		IPv4:       tfutil.StringValue(model.IPv4),
		IPv4Subnet: tfutil.StringValue(model.IPv4Subnet),
		IPv6:       tfutil.StringValue(model.IPv6),
		IPv6Subnet: tfutil.StringValue(model.IPv6Subnet),
		MAC:        tfutil.StringValue(model.MAC),
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
		LifetimeHours: tfutil.Int64Value(model.LifetimeHours),
	}
	for _, node := range model.Nodes {
		nodeSpec := topologybuilder.NodeSpec{
			Name:  node.Name.ValueString(),
			Site:  node.Site.ValueString(),
			Cores: tfutil.Int64Value(node.Cores),
			RAM:   tfutil.Int64Value(node.RAM),
			Disk:  tfutil.Int64Value(node.Disk),
		}
		for _, component := range node.Components {
			nodeSpec.Components = append(nodeSpec.Components, topologybuilder.ComponentSpec{
				Type:  sliver.ComponentType(tfutil.StringValue(component.Type)),
				Model: tfutil.StringValue(component.Model),
			})
		}
		spec.Nodes = append(spec.Nodes, nodeSpec)
	}
	for _, network := range model.Networks {
		spec.Networks = append(spec.Networks, topologybuilder.NetworkSpec{
			Type:      sliver.ServiceType(network.Type.ValueString()),
			Bandwidth: tfutil.Int64Value(network.Bandwidth),
		})
	}
	return topologybuilder.PermissionRequest(spec)
}
