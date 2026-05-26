package permission

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

type Request struct {
	ProjectTags   map[string]bool
	LifetimeHours int64
	Nodes         []Node
	Networks      []Network
}

type Node struct {
	Name       string
	Site       string
	Cores      int64
	RAM        int64
	Disk       int64
	Components []Component
}

type Component struct {
	Type  string
	Model string
}

type Network struct {
	Type      string
	Bandwidth int64
}

func Validate(ctx context.Context, req Request, diags *diag.Diagnostics) {
	_ = ctx
	if req.ProjectTags == nil {
		req.ProjectTags = map[string]bool{}
	}
	if req.LifetimeHours > 24 {
		require(diags, req.ProjectTags, path.Root("lifetime_hours"), TagSliceNoLimitLifetime)
	}
	sites := map[string]bool{}
	for i, node := range req.Nodes {
		if node.Site != "" {
			sites[node.Site] = true
		}
		nodePath := path.Root("node").AtListIndex(i)
		largeComposite := node.Cores > 64 || node.RAM > 384 || node.Disk > 1000
		if largeComposite {
			require(diags, req.ProjectTags, nodePath, TagVMNoLimit)
		}
		if node.Cores > 2 {
			require(diags, req.ProjectTags, nodePath.AtName("cores"), TagVMNoLimitCPU)
		}
		if node.RAM > 8 {
			require(diags, req.ProjectTags, nodePath.AtName("ram"), TagVMNoLimitRAM)
		}
		if node.Disk > 10 {
			require(diags, req.ProjectTags, nodePath.AtName("disk"), TagVMNoLimitDisk)
		}
		for j, component := range node.Components {
			componentPath := nodePath.AtName("component").AtListIndex(j)
			switch component.Type {
			case "GPU":
				require(diags, req.ProjectTags, componentPath.AtName("type"), TagComponentGPU)
			case "FPGA":
				require(diags, req.ProjectTags, componentPath.AtName("type"), TagComponentFPGA)
			case "NVME":
				require(diags, req.ProjectTags, componentPath.AtName("type"), TagComponentNVME)
			case "Storage":
				require(diags, req.ProjectTags, componentPath.AtName("type"), TagComponentStorage)
			case "SmartNIC":
				require(diags, req.ProjectTags, componentPath.AtName("model"), smartNICTag(component.Model))
			}
		}
	}
	if len(sites) > 1 {
		require(diags, req.ProjectTags, path.Root("node"), TagSliceMultisite)
	}
	for i, network := range req.Networks {
		networkPath := path.Root("network").AtListIndex(i)
		if network.Bandwidth > 10000 {
			require(diags, req.ProjectTags, networkPath.AtName("bandwidth"), TagNetNoLimitBW)
		}
		switch network.Type {
		case "FABNetv4Ext":
			require(diags, req.ProjectTags, networkPath.AtName("type"), TagNetFABNetv4Ext)
		case "FABNetv6Ext":
			require(diags, req.ProjectTags, networkPath.AtName("type"), TagNetFABNetv6Ext)
		case "PortMirror":
			require(diags, req.ProjectTags, networkPath.AtName("type"), TagNetPortMirroring)
		}
	}
}

func require(diags *diag.Diagnostics, tags map[string]bool, attr path.Path, tag string) {
	if tag == "" || tags[tag] {
		return
	}
	diags.AddAttributeError(
		attr,
		"Missing FABRIC project tag",
		"This configuration requires project tag "+tag+". Add the tag to the FABRIC project or reduce the requested resource.",
	)
}

func smartNICTag(model string) string {
	normalized := strings.ReplaceAll(model, "-", "_")
	switch normalized {
	case "ConnectX_5":
		return TagComponentSmartNICConnectX5
	case "ConnectX_6", "BlueField_2_ConnectX_6":
		return TagComponentSmartNICConnectX6
	case "ConnectX_7_100":
		return TagComponentSmartNICConnectX7100
	case "ConnectX_7_400":
		return TagComponentSmartNICConnectX7400
	default:
		return TagComponentSmartNICConnectX6
	}
}
