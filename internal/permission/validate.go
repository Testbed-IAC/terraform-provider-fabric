package permission

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

type Request struct {
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

func Validate(ctx context.Context, req Request, ts fabricclient.TokenSource, diags *diag.Diagnostics) {
	_ = ctx
	if req.LifetimeHours > 24 {
		require(diags, ts, path.Root("lifetime_hours"), TagSliceNoLimitLifetime)
	}
	sites := map[string]bool{}
	for i, node := range req.Nodes {
		if node.Site != "" {
			sites[node.Site] = true
		}
		nodePath := path.Root("node").AtListIndex(i)
		if node.Cores > 64 || node.RAM > 384 || node.Disk > 1000 {
			require(diags, ts, nodePath, TagVMNoLimit)
		}
		if node.Cores > 2 {
			require(diags, ts, nodePath.AtName("cores"), TagVMNoLimitCPU)
		}
		if node.RAM > 8 {
			require(diags, ts, nodePath.AtName("ram"), TagVMNoLimitRAM)
		}
		if node.Disk > 10 {
			require(diags, ts, nodePath.AtName("disk"), TagVMNoLimitDisk)
		}
		for j, component := range node.Components {
			componentPath := nodePath.AtName("component").AtListIndex(j)
			switch component.Type {
			case "GPU":
				require(diags, ts, componentPath.AtName("type"), TagComponentGPU)
			case "FPGA":
				require(diags, ts, componentPath.AtName("type"), TagComponentFPGA)
			case "NVME":
				require(diags, ts, componentPath.AtName("type"), TagComponentNVME)
			case "Storage":
				require(diags, ts, componentPath.AtName("type"), TagComponentStorage)
			case "SmartNIC":
				require(diags, ts, componentPath.AtName("model"), smartNICTag(component.Model))
			}
		}
	}
	if len(sites) > 1 {
		require(diags, ts, path.Root("node"), TagSliceMultisite)
	}
	for i, network := range req.Networks {
		networkPath := path.Root("network").AtListIndex(i)
		if network.Bandwidth > 10000 {
			require(diags, ts, networkPath.AtName("bandwidth"), TagNetNoLimitBW)
		}
		switch network.Type {
		case "FABNetv4Ext":
			require(diags, ts, networkPath.AtName("type"), TagNetFABNetv4Ext)
		case "FABNetv6Ext":
			require(diags, ts, networkPath.AtName("type"), TagNetFABNetv6Ext)
		case "PortMirror":
			require(diags, ts, networkPath.AtName("type"), TagNetPortMirroring)
		}
	}
}

func require(diags *diag.Diagnostics, ts fabricclient.TokenSource, attr path.Path, tag string) {
	if tag == "" {
		return
	}
	var claims *fabricclient.FabricClaims
	if ts != nil {
		claims = ts.Claims()
	}
	if claims.HasTag(tag) {
		return
	}
	projectName := "unknown project"
	if name := claims.Project().Name; name != "" {
		projectName = name
	}
	diags.AddAttributeError(
		attr,
		"Missing FABRIC project tag",
		fmt.Sprintf("This configuration requires project tag %q, but project %q does not have it in the token claims. Ask a FABRIC project lead to add the tag at https://portal.fabric-testbed.net/projects, then request a fresh token.", tag, projectName),
	)
}

func smartNICTag(model string) string {
	normalized := strings.ReplaceAll(model, "-", "_")
	switch normalized {
	case "ConnectX_5":
		return TagComponentSmartNICConnectX5
	case "ConnectX_6":
		return TagComponentSmartNICConnectX6
	case "BlueField_2_ConnectX_6":
		return TagComponentSmartNICBlueField2ConnectX6
	case "ConnectX_7_100":
		return TagComponentSmartNICConnectX7100
	case "ConnectX_7_400":
		return TagComponentSmartNICConnectX7400
	default:
		return TagComponentSmartNICConnectX6
	}
}
