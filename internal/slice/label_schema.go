package slice

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

type labelsModel struct {
	VLAN           types.String `tfsdk:"vlan"`
	VLANRange      types.String `tfsdk:"vlan_range"`
	InnerVLAN      types.String `tfsdk:"inner_vlan"`
	IPv4           types.String `tfsdk:"ipv4"`
	IPv4Range      types.String `tfsdk:"ipv4_range"`
	IPv4Subnet     types.String `tfsdk:"ipv4_subnet"`
	IPv6           types.String `tfsdk:"ipv6"`
	IPv6Range      types.String `tfsdk:"ipv6_range"`
	IPv6Subnet     types.String `tfsdk:"ipv6_subnet"`
	MAC            types.String `tfsdk:"mac"`
	ASN            types.String `tfsdk:"asn"`
	BGPKey         types.String `tfsdk:"bgp_key"`
	AccountID      types.String `tfsdk:"account_id"`
	Region         types.String `tfsdk:"region"`
	LocalName      types.String `tfsdk:"local_name"`
	LocalType      types.String `tfsdk:"local_type"`
	DeviceName     types.String `tfsdk:"device_name"`
	NUMA           types.Int64  `tfsdk:"numa"`
	BDF            types.String `tfsdk:"bdf"`
	USBID          types.String `tfsdk:"usb_id"`
	Instance       types.String `tfsdk:"instance"`
	InstanceParent types.String `tfsdk:"instance_parent"`
}

func labelsAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		Description:         "FABRIC/FIM labels applied to this topology element. Labels are optional placement, addressing, VLAN, and device hints interpreted by FABRIC.",
		MarkdownDescription: "FABRIC/FIM labels applied to this topology element. Labels are optional placement, addressing, VLAN, and device hints interpreted by FABRIC.",
		Attributes: map[string]schema.Attribute{
			"vlan":            labelStringAttribute("VLAN tag in the range 0 through 4096.", "vlan", false),
			"vlan_range":      labelStringAttribute(`VLAN range in "lo-hi" form.`, "vlan_range", false),
			"inner_vlan":      labelStringAttribute("Inner VLAN tag in the range 0 through 4096.", "inner_vlan", false),
			"ipv4":            labelStringAttribute("IPv4 address label.", "ipv4", false),
			"ipv4_range":      labelStringAttribute(`IPv4 address range in "lo-hi" form.`, "ipv4_range", false),
			"ipv4_subnet":     labelStringAttribute("IPv4 subnet label in CIDR form.", "ipv4_subnet", false),
			"ipv6":            labelStringAttribute("IPv6 address label.", "ipv6", false),
			"ipv6_range":      labelStringAttribute(`IPv6 address range in "lo-hi" form.`, "ipv6_range", false),
			"ipv6_subnet":     labelStringAttribute("IPv6 subnet label in CIDR form.", "ipv6_subnet", false),
			"mac":             labelStringAttribute("MAC address label.", "mac", false),
			"asn":             labelStringAttribute("Autonomous system number in the range 1 through 4294967295.", "asn", false),
			"bgp_key":         labelStringAttribute("BGP peering key. This value is sensitive, masked in plan output and state, and requires asn to be set.", "bgp_key", true),
			"account_id":      labelStringAttribute("External account identifier label.", "account_id", false),
			"region":          labelStringAttribute("External region label.", "region", false),
			"local_name":      labelStringAttribute("Local port or device name label.", "local_name", false),
			"local_type":      labelStringAttribute("Local port or device type label.", "local_type", false),
			"device_name":     labelStringAttribute("Device name label.", "device_name", false),
			"numa":            schema.Int64Attribute{Optional: true, Description: "NUMA label in the range -1 through 7.", MarkdownDescription: "NUMA label in the range `-1` through `7`.", Validators: []validator.Int64{int64validator.Between(-1, 7)}},
			"bdf":             labelStringAttribute("PCI bus-device-function label.", "bdf", false),
			"usb_id":          labelStringAttribute("USB identifier label.", "usb_id", false),
			"instance":        labelStringAttribute("FABRIC instance label.", "instance", false),
			"instance_parent": labelStringAttribute("FABRIC instance parent label used for host pinning.", "instance_parent", false),
		},
	}
}

func labelStringAttribute(description, field string, sensitive bool) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		Sensitive:           sensitive,
		Description:         description,
		MarkdownDescription: description,
		Validators:          []validator.String{labelStringValidator{field: field}},
	}
}

func (m labelsModel) toFIM() (*sliver.Labels, error) {
	labels := sliver.Labels{
		BDF:            tfutil.StringValue(m.BDF),
		MAC:            tfutil.StringValue(m.MAC),
		IPv4:           tfutil.StringValue(m.IPv4),
		IPv4Range:      tfutil.StringValue(m.IPv4Range),
		IPv4Subnet:     tfutil.StringValue(m.IPv4Subnet),
		IPv6:           tfutil.StringValue(m.IPv6),
		IPv6Range:      tfutil.StringValue(m.IPv6Range),
		IPv6Subnet:     tfutil.StringValue(m.IPv6Subnet),
		VLAN:           tfutil.StringValue(m.VLAN),
		VLANRange:      tfutil.StringValue(m.VLANRange),
		InnerVLAN:      tfutil.StringValue(m.InnerVLAN),
		ASN:            tfutil.StringValue(m.ASN),
		Instance:       tfutil.StringValue(m.Instance),
		InstanceParent: tfutil.StringValue(m.InstanceParent),
		LocalName:      tfutil.StringValue(m.LocalName),
		LocalType:      tfutil.StringValue(m.LocalType),
		DeviceName:     tfutil.StringValue(m.DeviceName),
		BGPKey:         tfutil.StringValue(m.BGPKey),
		AccountID:      tfutil.StringValue(m.AccountID),
		Region:         tfutil.StringValue(m.Region),
		USBID:          tfutil.StringValue(m.USBID),
	}
	if !m.NUMA.IsNull() && !m.NUMA.IsUnknown() {
		numa := int(m.NUMA.ValueInt64())
		labels.NUMA = &numa
	}
	if labels.Empty() {
		return nil, nil
	}
	if err := labels.Validate(); err != nil {
		return nil, fmt.Errorf("validating labels: %w", err)
	}
	return &labels, nil
}

func labelsToFIM(m *labelsModel) (*sliver.Labels, error) {
	if m == nil {
		return nil, nil
	}
	return m.toFIM()
}

func validateLabelConfiguration(model SliceResourceModel, diags *diag.Diagnostics) {
	for i, node := range model.Nodes {
		nodePath := path.Root("node").AtListIndex(i)
		validateLabelsBlock(node.Labels, nodePath.AtName("labels"), diags)
		validateNodeHost(node, nodePath, diags)
		for j, component := range node.Components {
			validateLabelsBlock(component.Labels, nodePath.AtName("component").AtListIndex(j).AtName("labels"), diags)
		}
	}
	for i, network := range model.Networks {
		networkPath := path.Root("network").AtListIndex(i)
		validateLabelsBlock(network.Labels, networkPath.AtName("labels"), diags)
		for j, iface := range network.Interfaces {
			ifacePath := networkPath.AtName("interface").AtListIndex(j)
			validateLabelsBlock(iface.Labels, ifacePath.AtName("labels"), diags)
			for k, sub := range iface.SubInterfaces {
				validateLabelsBlock(sub.Labels, ifacePath.AtName("sub_interface").AtListIndex(k).AtName("labels"), diags)
			}
		}
	}
	for i, facility := range model.Facilities {
		facilityPath := path.Root("facility_port").AtListIndex(i)
		validateLabelsBlock(facility.Labels, facilityPath.AtName("labels"), diags)
		for j, iface := range facility.Interfaces {
			validateLabelsBlock(iface.Labels, facilityPath.AtName("interface").AtListIndex(j).AtName("labels"), diags)
		}
	}
	for i, sw := range model.Switches {
		validateLabelsBlock(sw.PortLabels, path.Root("switch").AtListIndex(i).AtName("port_labels"), diags)
	}
}

func validateLabelsBlock(labels *labelsModel, labelsPath path.Path, diags *diag.Diagnostics) {
	fimLabels, err := labelsToFIM(labels)
	if err != nil {
		if errors.Is(err, sliver.ErrBGPKeyRequiresASN) {
			diags.AddAttributeError(
				labelsPath.AtName("bgp_key"),
				"Invalid FABRIC BGP labels",
				fmt.Sprintf("bgp_key requires asn in the same labels block. Set %s or remove %s.", labelsPath.AtName("asn"), labelsPath.AtName("bgp_key")),
			)
			return
		}
		diags.AddAttributeError(labelsPath, "Invalid FABRIC labels", "One or more labels are invalid. Correct the label values and run plan again. Original error: "+err.Error())
		return
	}
	_ = fimLabels
}

func validateNodeHost(node NodeModel, nodePath path.Path, diags *diag.Diagnostics) {
	host := tfutil.StringValue(node.Host)
	instanceParent := ""
	if node.Labels != nil {
		instanceParent = tfutil.StringValue(node.Labels.InstanceParent)
	}
	if host == "" || instanceParent == "" || host == instanceParent {
		return
	}
	diags.AddAttributeError(
		nodePath.AtName("host"),
		"Conflicting FABRIC host labels",
		fmt.Sprintf("node.host sets labels.instance_parent to %q, but %s is %q. Set only one value or make them match.", host, nodePath.AtName("labels").AtName("instance_parent"), instanceParent),
	)
}

type labelStringValidator struct {
	field string
}

func (v labelStringValidator) Description(context.Context) string {
	return "must be a valid FABRIC label value"
}

func (v labelStringValidator) MarkdownDescription(context.Context) string {
	return "must be a valid FABRIC label value"
}

func (v labelStringValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	_ = ctx
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	labels := sliver.Labels{}
	setLabelField(&labels, v.field, req.ConfigValue.ValueString())
	if err := labels.Validate(); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid FABRIC label",
			"The label value is not valid for FABRIC/FIM. Original error: "+err.Error(),
		)
	}
}

func setLabelField(labels *sliver.Labels, field, value string) {
	switch field {
	case "bdf":
		labels.BDF = value
	case "mac":
		labels.MAC = value
	case "ipv4":
		labels.IPv4 = value
	case "ipv4_range":
		labels.IPv4Range = value
	case "ipv4_subnet":
		labels.IPv4Subnet = value
	case "ipv6":
		labels.IPv6 = value
	case "ipv6_range":
		labels.IPv6Range = value
	case "ipv6_subnet":
		labels.IPv6Subnet = value
	case "vlan":
		labels.VLAN = value
	case "vlan_range":
		labels.VLANRange = value
	case "inner_vlan":
		labels.InnerVLAN = value
	case "asn":
		labels.ASN = value
	case "bgp_key":
		labels.BGPKey = value
	case "account_id":
		labels.AccountID = value
	case "region":
		labels.Region = value
	case "usb_id":
		labels.USBID = value
	}
}
